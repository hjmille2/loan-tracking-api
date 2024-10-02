package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// TO BE REFACTORED
var db *sql.DB

type loan struct {
	Loan_ID             int     `json:"loan_id"`
	Nickname            string  `json:"nickname"`
	Starting_Amount     float64 `json:"starting_amount"`
	Interest_Rate       float32 `json:"interest_rate"`
	Current_Amount_Owed float64 `json:"current_amount_owed"`
	Description         string  `json:"description"`
}

type loanUpdate struct {
	Nickname      string  `json:"nickname"`
	Interest_Rate float32 `json:"interest_rate"`
	Description   string  `json:"description"`
}

type payment struct {
	Payment_ID     int     `json:"payment_id"`
	Loan_ID        int     `json:"loan_id"`
	Payment_Date   string  `json:"payment_date"`
	Principal_Paid float32 `json:"principal_paid"`
	Interest_Paid  float32 `json:"interest_paid"`
}

func getEnvVar(key string) string {
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatal("Error loading .env file.")
	}

	return os.Getenv(key)
}

func connectDB() *sql.DB {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		getEnvVar("host"), getEnvVar("port"), getEnvVar("user"), getEnvVar("password"), getEnvVar("dbname"))

	db, err := sql.Open("postgres", connStr)

	if err != nil {
		panic(err)
	}

	return db
}

func getAllLoans(c *gin.Context) {
	c.Header("Content-Type", "application/json")

	queryString := `SELECT loan_id, nickname, starting_amount, interest_rate, current_amount_owed, description FROM loan`
	rows, err := db.Query(queryString)

	//check for error
	if err != nil {
		log.Fatal(err)
	}

	defer rows.Close()

	//initializing this way as i dont want a null value returned
	loans := []loan{}
	for rows.Next() {
		var l loan
		err := rows.Scan(&l.Loan_ID, &l.Nickname, &l.Starting_Amount, &l.Interest_Rate, &l.Current_Amount_Owed, &l.Description)
		if err != nil {
			log.Fatal(err)
		}
		loans = append(loans, l)
	}

	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}

	c.JSON(http.StatusOK, loans)
}

func getLoanByID(c *gin.Context) {
	id := c.Param("id")
	c.Header("Content-Type", "application/json")

	queryString := `
		SELECT loan_id, nickname, starting_amount, interest_rate, current_amount_owed, description FROM loan 
		WHERE loan_id=$1`

	var loan loan

	if err := db.QueryRow(queryString, id).Scan(&loan); err != nil {
		switch err {
		case sql.ErrNoRows:
			c.JSON(http.StatusNotFound, "No loan with that ID exists.")
			return
		default:
			log.Fatal(err)
		}

	}

	c.JSON(http.StatusOK, loan)

}

func createNewLoan(c *gin.Context) {
	var newLoan loan
	if err := c.BindJSON(&newLoan); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload."})
		return
	}

	queryString := `
		INSERT INTO loan (nickname, starting_amount, interest_rate, current_amount_owed, description) 
		VALUES ($1, $2, $3, $4, $5)
		RETURNING loan_id, nickname, starting_amount, interest_rate, current_amount_owed, description
	`

	var loanCreated loan

	if err := db.QueryRow(queryString, newLoan.Nickname, newLoan.Starting_Amount, newLoan.Interest_Rate, newLoan.Current_Amount_Owed, newLoan.Description).Scan(
		&loanCreated.Loan_ID, &loanCreated.Nickname, &loanCreated.Starting_Amount, &loanCreated.Interest_Rate, &loanCreated.Current_Amount_Owed, &loanCreated.Description); err != nil {
		log.Fatal(err)
	}

	c.JSON(http.StatusCreated, loanCreated)
}

func main() {
	db = connectDB()

	router := gin.Default()

	//LOAN ROUTES
	router.GET("/loans", getAllLoans)
	router.GET("/loans/:id", getLoanByID)
	router.POST("/loans", createNewLoan)

	//Open Connection
	router.Run("localhost:8080")

	defer db.Close()
}
