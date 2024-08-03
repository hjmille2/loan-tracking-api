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

	print(connStr)
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

	c.IndentedJSON(http.StatusOK, loans)
}

func main() {
	db = connectDB()

	router := gin.Default()

	//LOAN ROUTES
	router.GET("/loans", getAllLoans)

	//Open Connection
	router.Run("localhost:8080")

	defer db.Close()
}
