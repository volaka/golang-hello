package main

import (
	"bytes"
	"encoding/json"
	"github.com/joho/godotenv"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

func TestCheckEnvironment(t *testing.T) {
	// Backup original environment variables
	originalEnv := map[string]string{
		"DB_HOST":     os.Getenv("DB_HOST"),
		"DB_PORT":     os.Getenv("DB_PORT"),
		"DB_USER":     os.Getenv("DB_USER"),
		"DB_PASSWORD": os.Getenv("DB_PASSWORD"),
		"DB_NAME":     os.Getenv("DB_NAME"),
	}

	// Restore original environment variables after the test
	defer func() {
		for key, value := range originalEnv {
			os.Setenv(key, value)
		}
	}()

	t.Run("Missing environment variables", func(t *testing.T) {
		os.Clearenv()

		err := checkEnvironment()
		if err == nil {
			t.Errorf("Expected error due to missing environment variables, but got nil")
		}
	})

	t.Run("All required environment variables set", func(t *testing.T) {
		os.Setenv("DB_HOST", "localhost")
		os.Setenv("DB_PORT", "5432")
		os.Setenv("DB_USER", "testuser")
		os.Setenv("DB_PASSWORD", "testpassword")
		os.Setenv("DB_NAME", "testdb")

		err := checkEnvironment()
		if err != nil {
			t.Errorf("Expected no error, but got %v", err)
		}
	})
}

func TestLoadEnv(t *testing.T) {
	// Backup original environment variables
	originalEnv := map[string]string{
		"DB_HOST":     os.Getenv("DB_HOST"),
		"DB_PORT":     os.Getenv("DB_PORT"),
		"DB_USER":     os.Getenv("DB_USER"),
		"DB_PASSWORD": os.Getenv("DB_PASSWORD"),
		"DB_NAME":     os.Getenv("DB_NAME"),
		"PORT":        os.Getenv("PORT"),
		"ENVIRONMENT": os.Getenv("ENVIRONMENT"),
	}

	// Restore original environment variables after the test
	defer func() {
		for key, value := range originalEnv {
			os.Setenv(key, value)
		}
	}()

	// test for non existing .env file
	t.Run("Non-existing .env file", func(t *testing.T) {
		// backup original .env file
		os.Rename(".env", ".env.bak")
		// remove .env file
		os.Remove(".env")
		defer func() {
			// restore original .env file
			os.Rename(".env.bak", ".env")
		}()

		os.Clearenv()
		defer godotenv.Load()

		err := loadEnv()
		if err == nil {
			t.Errorf("Expected error, got nil")
		}

		// check error message
		if err.Error() != "error loading .env file: open .env: no such file or directory" {
			t.Errorf("Expected error message to be 'error loading .env file: open .env: no such file or directory', got %q", err.Error())
		}
	})

	t.Run("Missing environment variables", func(t *testing.T) {
		os.Clearenv()
		defer godotenv.Load() // Reload .env file after test

		// replace .env file with one missing the required environment variables
		os.Rename(".env", ".env.bak")
		defer os.Rename(".env.bak", ".env") // Restore original .env file after test

		// create a new .env file with missing environment variables
		f, err := os.Create(".env")
		if err != nil {
			t.Fatalf("Failed to create .env file: %v", err)
		}
		defer f.Close()

		// populate env file
		_, err = f.WriteString("PORT=8080\n")
		if err != nil {
			t.Fatalf("Failed to write to .env file: %v", err)
		}

		err = loadEnv()
		if err == nil {
			t.Errorf("Expected error due to missing environment variables, but got nil")
		}
	})

	t.Run("Successful environment variable loading", func(t *testing.T) {
		os.Clearenv()
		// create .env file with all required environment variables
		f, err := os.Create(".env")
		if err != nil {
			t.Fatalf("Failed to create .env file: %v", err)
		}
		defer f.Close()

		_, err = f.WriteString("DB_HOST=db\nDB_USER=volaka\nDB_PASSWORD=volaka_password\nDB_NAME=volaka\nDB_PORT=5432\nPORT=8080\nPOSTGRES_USER=volaka\nPOSTGRES_PASSWORD=volaka_password\nPOSTGRES_DB=volaka\n")
		if err != nil {
			t.Fatalf("Failed to write to .env file: %v", err)
		}

		err = loadEnv()
		if err != nil {
			t.Errorf("Expected error due to missing environment variables, but got nil")
		}

	})
}

func TestInitDB(t *testing.T) {
	// Backup original environment variables
	// Backup original environment variables
	originalEnv := map[string]string{
		"DB_HOST":     os.Getenv("DB_HOST"),
		"DB_PORT":     os.Getenv("DB_PORT"),
		"DB_USER":     os.Getenv("DB_USER"),
		"DB_PASSWORD": os.Getenv("DB_PASSWORD"),
		"DB_NAME":     os.Getenv("DB_NAME"),
	}

	// Restore original environment variables after the test
	defer func() {
		for key, value := range originalEnv {
			os.Setenv(key, value)
		}
	}()

	t.Run("Successful database connection and migration", func(t *testing.T) {
		os.Clearenv()
		defer godotenv.Load()
		godotenv.Load(".env.local")

		initDB()

		if DB == nil {
			t.Fatalf("Expected db to be initialized, but it was nil")
		}

		// Check if the database connection is valid
		sqlDB, err := DB.DB()
		if err != nil {
			t.Fatalf("Failed to get database connection: %v", err)
		}
		defer sqlDB.Close()

		if err := sqlDB.Ping(); err != nil {
			t.Fatalf("Failed to ping database: %v", err)
		}
	})
}

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name            string
		username        string
		expectedError   bool
		expectedMessage string
		expectedStatus  int
	}{
		{"Empty username", "", true, "Username cannot be empty.", http.StatusBadRequest},
		{"Valid username", "Alice", false, "", 0},
		{"Username too long", string(make([]byte, 256)), true, "Username is too long. Maximum length is 255 characters.", http.StatusBadRequest},
		{"Invalid username with numbers", "Alice123", true, "Invalid username. Only letters are allowed.", http.StatusBadRequest},
		{"Invalid username with special characters", "Alice!", true, "Invalid username. Only letters are allowed.", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateUsername(tt.username)
			if result.Error != tt.expectedError {
				t.Errorf("Expected error %v, got %v", tt.expectedError, result.Error)
			}
			if result.Message != tt.expectedMessage {
				t.Errorf("Expected message %q, got %q", tt.expectedMessage, result.Message)
			}
			if result.Status != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, result.Status)
			}
		})
	}
}

func TestSaveUser(t *testing.T) {
	os.Clearenv()
	defer godotenv.Load()
	godotenv.Load(".env.local")

	initDB()
	// Test cases
	tests := []struct {
		name           string
		username       string
		dateOfBirth    string
		expectedStatus int
	}{
		{"Valid user", "Alice", "1990-05-15", http.StatusNoContent},
		{"Invalid username", "Alice123", "1990-05-15", http.StatusBadRequest},
		{"Invalid date of birth (future)", "Bob", "2042-01-01", http.StatusBadRequest},
		{"Invalid date of birth (format)", "Carol", "05-15-1995", http.StatusBadRequest},
		{"Invalid date of birth (format)", "Carol", "05-15-1995", http.StatusBadRequest},
		{"Updating existing user", "Alice", "1991-05-15", http.StatusNoContent},
		{"Invalid body", "Alice", "", http.StatusBadRequest},
	}
	// Clean up the database before test
	DB.Exec("DELETE FROM users")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestBody := map[string]string{"dateOfBirth": tt.dateOfBirth}
			jsonBody, _ := json.Marshal(requestBody)

			req := httptest.NewRequest(http.MethodPost, "/hello/"+tt.username, bytes.NewBuffer(jsonBody))
			w := httptest.NewRecorder()

			router := mux.NewRouter()
			router.HandleFunc("/hello/{username}", saveUser)
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestGetBirthdayMessage(t *testing.T) {
	os.Clearenv()
	defer godotenv.Load()
	godotenv.Load(".env.local")

	initDB()
	// Clean up the database before the test
	DB.Exec("DELETE FROM users")

	// Seed test data
	DB.Create(&User{Name: "David", DateOfBirth: "1985-12-25"})

	tests := []struct {
		name           string
		username       string
		expectedStatus int
	}{
		{
			name:           "User found",
			username:       "David",
			expectedStatus: http.StatusOK,
		},
		{
			"User not found",
			"Frank",
			http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/hello/"+tt.username, nil)
			w := httptest.NewRecorder()

			router := mux.NewRouter()
			router.HandleFunc("/hello/{username}", getBirthdayMessage)
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestCalculateDaysUntilBirthday(t *testing.T) {
	today := time.Date(2024, time.July, 25, 0, 0, 0, 0, time.UTC)
	// Test cases
	tests := []struct {
		name           string
		dateOfBirth    string
		today          time.Time
		expectedResult int
	}{
		{"Same day", "1990-07-25", today, 0},
		{"Next day", "1990-07-26", today, 1},
		{"Next year", "1990-07-24", today, 364},
		{"Leap year birthday", "1992-02-29", time.Date(2024, time.February, 28, 0, 0, 0, 0, time.UTC), 1},
		{"End of year birthday", "1990-12-31", today, 159},
		{"Start of year birthday", "1990-01-01", today, 160},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateDaysUntilBirthday(tt.dateOfBirth, tt.today)
			if result != tt.expectedResult {
				t.Errorf("Expected result %d, got %d", tt.expectedResult, result)
			}
		})
	}
}
