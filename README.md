# Golang Hello Project

## Description

The Golang Hello project is a cloud-native application designed to run on AWS ECS using Fargate. It leverages AWS CDK for infrastructure as code, and implements a blue/green deployment strategy using AWS CodeDeploy to ensure zero downtime during deployments. The application is written in Go and uses PostgreSQL as its database.

## Local Run Guidance

1. **Clone the repository and change into the directory**:
    - Open your terminal.
    - Run the following commands to clone the repository and navigate into the project directory:
    ```sh
    git clone https://github.com/volaka/golang-hello.git
    cd golang-hello
    ```

2. **Create `.env.local` file**:
    - The application requires environment variables to run. These variables should be defined in a `.env` file.
    - Create a `.env.local` file in the root directory of the project and populate it with the necessary environment variables. Here is an example of what the `.env` file might look like:
    ```plaintext
    DB_HOST=localhost
    DB_USER=volaka
    DB_PASSWORD=volaka_password
    DB_NAME=volaka
    DB_PORT=5432
    PORT=8080
    POSTGRES_USER=volaka
    POSTGRES_PASSWORD=volaka_password
    POSTGRES_DB=volaka
    ```

3. **Run Docker Compose for local database stack**:
    - The application uses a PostgreSQL database, which can be run locally using Docker Compose.
    - Ensure Docker is installed and running on your machine.
    - Run the following command to start the local database stack:
    ```sh
    docker-compose -f docker-compose-local-db.yml up
    ```

4. **Run the application**:
    - With the database running, you can now start the application.
    - Run the following command to start the Go application:
    ```sh
    go run main.go
    ```

## How to Run Tests

1. **Create `.env.local` file**:
    - The tests require environment variables to run. These variables should be defined in a `.env.local` file.
    - Create a `.env.local` file in the root directory of the project and populate it with the necessary environment variables. Here is an example of what the `.env.local` file might look like:
    ```plaintext
    DB_HOST=localhost
    DB_USER=volaka
    DB_PASSWORD=volaka_password
    DB_NAME=volaka
    DB_PORT=5432
    PORT=8080
    POSTGRES_USER=volaka
    POSTGRES_PASSWORD=volaka_password
    POSTGRES_DB=volaka
    ```

2. **Run Docker Compose for test database stack**:
    - The tests use a PostgreSQL database, which can be run locally using Docker Compose.
    - Ensure Docker is installed and running on your machine.
    - Run the following command to start the test database stack:
    ```sh
    docker-compose -f docker-compose-local-db.yml up
    ```

3. **Run tests**:
    - The project uses Go's built-in testing framework.
    - To run all tests in the project, execute the following command:
    ```sh
    go test ./...
    ```
    - This command will recursively find and run all test files (`*_test.go`) in the project directories.

4. **Check test results**:
    - After running the tests, the results will be displayed in the terminal.
    - Review the output to ensure all tests have passed. If any tests fail, the output will provide details on the failures, which you can use to debug and fix the issues. 
    - Hub Actions workflow defined in `.github/workflows/cd.yaml`, which will build and deploy the application to AWS ECS using CodeDeploy.

## Deployment Instructions

