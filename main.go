package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	DB  *gorm.DB
	log = logrus.New()
	err error
)

type User struct {
	ID          uint   `gorm:"primaryKey"`
	Name        string `gorm:"unique"`
	DateOfBirth string
}

// initDB initializes the database connection and performs auto migration for the User model.
func initDB() {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	err = DB.AutoMigrate(&User{})
	if err != nil {
		log.Fatalf("Failed to migrate database schema: %v", err)
	}

	log.WithFields(logrus.Fields{
		"host":   host,
		"port":   port,
		"user":   user,
		"dbname": dbname,
	}).Debug("Database connection initialized")
}

func checkEnvironment() error {
	requiredVariables := []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME"}
	for _, variable := range requiredVariables {
		if os.Getenv(variable) == "" {
			return fmt.Errorf("Required environment variable %s is not set", variable)
		}
	}
	return nil
}

// loadEnv loads environment variables from .env file in development mode.
// It also checks for the required environment variables and terminates the program if any of them is missing.
func loadEnv() error {
	environment := os.Getenv("ENVIRONMENT")
	if environment != "PRODUCTION" {
		err := godotenv.Load(".env", ".env.local")
		if err != nil {
			return fmt.Errorf("error loading .env file: %v", err)
		}
	}

	err := checkEnvironment()
	if err != nil {
		return fmt.Errorf("error checking environment variables: %v", err)
	}
	log.Info("Environment variables loaded")
	return nil
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Infof("Method: %s, URL: %s, Duration: %s", r.Method, r.URL.Path, time.Since(start))
	})
}

type ValidationError struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

func validateUsername(username string) ValidationError {
	// Check if the username is empty
	if username == "" {
		log.Warn("Empty username")
		return ValidationError{Error: true, Message: "Username cannot be empty.", Status: http.StatusBadRequest}
	}

	// Check if the username is too long
	if len(username) > 255 {
		log.Warn("Username is too long")
		return ValidationError{Error: true, Message: "Username is too long. Maximum length is 255 characters.", Status: http.StatusBadRequest}
	}

	// Validate username
	if !regexp.MustCompile(`^[a-zA-Z]+$`).MatchString(username) {
		log.Warn("Invalid username")
		return ValidationError{Error: true, Message: "Invalid username. Only letters are allowed.", Status: http.StatusBadRequest}
	}

	return ValidationError{Error: false}
}

// saveUser handles the HTTP POST request to save or update a user.
// It validates the username, request body, and date of birth before saving the user to the database.
func saveUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	username := vars["username"]

	// validate username
	validationError := validateUsername(username)

	if validationError.Error {
		http.Error(w, validationError.Message, validationError.Status)
		return
	}

	// Parse request body
	var requestBody struct {
		DateOfBirth string `json:"dateOfBirth"`
	}

	// Check if date of birth is empty
	if r.Body == nil {
		log.Warn("Empty request body")
		http.Error(w, "Request body cannot be empty.", http.StatusBadRequest)
		return
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		log.Warn("Invalid request body")
		http.Error(w, "Invalid request body.", http.StatusBadRequest)
		return
	}

	// Validate date of birth
	dateOfBirth, err := time.Parse("2006-01-02", requestBody.DateOfBirth)
	if err != nil || dateOfBirth.After(time.Now()) {
		log.Warn("Invalid date of birth")
		http.Error(w, "Date of birth must be before today and in YYYY-MM-DD format.", http.StatusBadRequest)
		return
	}
	// Save or update user
	user := User{Name: username, DateOfBirth: dateOfBirth.Format("2006-01-02")}
	existingUser := User{}
	if result := DB.First(&existingUser, "name = ?", username); result.Error == nil {
		log.Infof("Existing user data: %+v", existingUser)
		log.Infof("New user data: %+v", user)
		existingUser.DateOfBirth = user.DateOfBirth
		DB.Save(&existingUser)
		log.Info("User updated successfully")
	} else if result.Error == gorm.ErrRecordNotFound {
		DB.Create(&user)
		log.Info("New user created successfully")
	} else {
		log.Warn("Error occurred while checking for existing user")
		http.Error(w, "Internal server error.", http.StatusInternalServerError)
		return
	}
	log.Info("User saved successfully")
	w.WriteHeader(http.StatusNoContent)
}

// getBirthdayMessage handles the HTTP GET request to retrieve the birthday message for a user.
// It retrieves the user from the database based on the username and calculates the days until the next birthday.
// It returns a JSON response with the birthday message.
func getBirthdayMessage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	username := vars["username"]

	var user User
	if result := DB.First(&user, "name = ?", username); result.Error != nil {
		log.Warn("User not found")
		http.Error(w, "User not found.", http.StatusNotFound)
		return
	}
	today := time.Now()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	daysUntilBirthday := calculateDaysUntilBirthday(user.DateOfBirth, today)

	var message string
	if daysUntilBirthday == 0 {
		message = fmt.Sprintf("Hello, %s! Happy birthday!", username)
	} else {
		message = fmt.Sprintf("Hello, %s! Your birthday is in %d day(s)", username, daysUntilBirthday)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": message})
}

func calculateDaysUntilBirthday(dateOfBirth string, today time.Time) int {
	birthday, _ := time.Parse("2006-01-02", dateOfBirth)
	birthday = birthday.AddDate(today.Year()-birthday.Year(), 0, 0)
	if birthday.Before(today) {
		birthday = birthday.AddDate(1, 0, 0)
	}
	return int(birthday.Sub(today).Hours() / 24)
}

func main() {
	if os.Getenv("DEBUG") != "" {
		log.SetLevel(logrus.DebugLevel) // Set log level to debug
	}
	loadEnv()
	initDB()

	psqlDB, err := DB.DB()
	if err != nil {
		log.Fatal("failed to get database connection")
	}
	defer psqlDB.Close() // Ensure the database connection is closed when main exits

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)                    // Create a channel to receive signals
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM) // Notify the channel on interrupt signals
	go func() {
		<-sigChan // Wait for the interrupt signal
		fmt.Println("Received interrupt signal, shutting down...")
		if err := psqlDB.Close(); err != nil {
			log.Fatal("Error closing database connection:", err)
		} else {
			log.Info("Database connection closed")
		}
		os.Exit(0)
	}()

	r := mux.NewRouter()
	r.Use(loggingMiddleware) // Add the logging middleware to the router
	r.HandleFunc("/hello/{username}", saveUser).Methods("POST")
	r.HandleFunc("/hello/{username}", getBirthdayMessage).Methods("GET")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Infof("Starting server on port %s", port)
	http.ListenAndServe(":"+port, r)
}
