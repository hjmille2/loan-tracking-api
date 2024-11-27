package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// TO BE REFACTORED
var db *sql.DB

type loan struct {
	Loan_ID             int     `json:"loan_id"`
	Nickname            string  `json:"nickname" binding:"required"`
	Starting_Amount     float64 `json:"starting_amount" binding:"required"`
	Interest_Rate       float32 `json:"interest_rate" binding:"required"`
	Current_Amount_Owed float64 `json:"current_amount_owed" binding:"required"`
	Description         string  `json:"description"`
}

type loanUpdate struct {
	Nickname      string  `json:"nickname" binding:"required"`
	Interest_Rate float32 `json:"interest_rate" binding:"required"`
	Description   string  `json:"description" binding:"required"`
}

type payment struct {
	Payment_ID     int     `json:"payment_id"`
	Payment_Date   string  `json:"payment_date" binding:"required"`
	Principal_Paid float32 `json:"principal_paid" binding:"required"`
	Interest_Paid  float32 `json:"interest_paid" binding:"required"`
}

type ErrorMessage struct {
	Field   string
	Message string
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
	id := c.Param("loanId")
	c.Header("Content-Type", "application/json")

	queryString := `
		SELECT loan_id, nickname, starting_amount, interest_rate, current_amount_owed, description FROM loan 
		WHERE loan_id=$1`

	var loan loan

	if err := db.QueryRow(queryString, id).Scan(&loan.Loan_ID, &loan.Nickname, &loan.Starting_Amount, &loan.Interest_Rate, &loan.Current_Amount_Owed, &loan.Description); err != nil {
		switch err {
		case sql.ErrNoRows:
			c.JSON(http.StatusNotFound, "No loan with that ID exists.")
			return
		default:
			c.JSON(http.StatusBadRequest, gin.H{"errors": err})
		}

	}

	c.JSON(http.StatusOK, loan)

}

func createNewLoan(c *gin.Context) {
	var newLoan loan

	//make sure response is json
	c.Header("Content-Type", "application/json")
	//error handling, make sure it aligns with the bindings of the struct and is valid json input
	if err := c.ShouldBindBodyWithJSON(&newLoan); err != nil {
		var validator validator.ValidationErrors
		if errors.As(err, &validator) {
			errorOutput := make([]ErrorMessage, len(validator))
			for i, indivErr := range validator {
				errorOutput[i] = ErrorMessage{indivErr.Field(), retErrorStr(indivErr.Tag())}
			}

			c.JSON(http.StatusBadRequest, gin.H{"errors": errorOutput})
			return
		}
	}

	//query to insert in, added returning as i want to retun these values to our uses to reduce the amount of querying
	queryString := `
		INSERT INTO loan (nickname, starting_amount, interest_rate, current_amount_owed, description) 
		VALUES ($1, $2, $3, $4, $5)
		RETURNING loan_id, nickname, starting_amount, interest_rate, current_amount_owed, description
	`
	var loanCreated loan
	if err := db.QueryRow(queryString, newLoan.Nickname, newLoan.Starting_Amount, newLoan.Interest_Rate, newLoan.Current_Amount_Owed, newLoan.Description).Scan(
		&loanCreated.Loan_ID, &loanCreated.Nickname, &loanCreated.Starting_Amount, &loanCreated.Interest_Rate, &loanCreated.Current_Amount_Owed, &loanCreated.Description); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": err})
		return
	}

	c.JSON(http.StatusCreated, loanCreated)
}

func updateLoanByID(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	id := c.Param("loanId")
	var loanUpdate loanUpdate
	//check that input is valid
	if err := c.ShouldBindBodyWithJSON(&loanUpdate); err != nil {
		var validator validator.ValidationErrors
		if errors.As(err, &validator) {
			errorOutput := make([]ErrorMessage, len(validator))
			for i, indivErr := range validator {
				errorOutput[i] = ErrorMessage{indivErr.Field(), retErrorStr(indivErr.Tag())}
			}

			c.JSON(http.StatusBadRequest, gin.H{"errors": errorOutput})
			return
		}
	}

	queryString := `
		UPDATE loan
		SET
			nickname = $1,
			interest_rate = $2,
			description = $3
		WHERE loan_id = $4
		RETURNING loan_id, nickname, starting_amount, interest_rate, current_amount_owed, description
	`
	var updatedLoan loan
	if err := db.QueryRow(queryString, loanUpdate.Nickname, loanUpdate.Interest_Rate, loanUpdate.Description, id).Scan(
		&updatedLoan.Loan_ID, &updatedLoan.Nickname, &updatedLoan.Starting_Amount, &updatedLoan.Interest_Rate, &updatedLoan.Current_Amount_Owed, &updatedLoan.Description); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": err})
		return
	}

	c.JSON(http.StatusOK, updatedLoan)
}

// this will delete all rows in payments that have this loan id as foreign key
// along with deleting in the loan table
// it is irreversible
func deleteLoanDataByID(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	id := c.Param("loanId")

	deleteFromPaymentQueryString := `DELETE FROM payment WHERE loan_id=$1`
	deleteFromLoanQueryString := `DELETE FROM loan WHERE loan_id=$1`

	if _, err := db.Exec(deleteFromPaymentQueryString, id); err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"errors": err})
		return
	}

	if _, err := db.Exec(deleteFromLoanQueryString, id); err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"errors": err})
		return
	}

	c.JSON(http.StatusOK, "Successfully deleted loan with id "+id)

}

func createNewPayment(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	var newPayment payment
	loanId := c.Param("loanId")

	if err := c.ShouldBindBodyWithJSON(&newPayment); err != nil {
		var validator validator.ValidationErrors
		if errors.As(err, &validator) {
			errorOutput := make([]ErrorMessage, len(validator))
			for i, indivErr := range validator {
				errorOutput[i] = ErrorMessage{indivErr.Field(), retErrorStr(indivErr.Tag())}
			}

			c.JSON(http.StatusBadRequest, gin.H{"errors": errorOutput})
			return
		}
	}

	queryString := `
		INSERT INTO payment (loan_id, payment_date, principal_paid, interest_paid)
		VALUES ($1, $2, $3, $4)
		RETURNING payment_id, payment_date, principal_paid, interest_paid
	`

	var paymentCreated payment
	if err := db.QueryRow(queryString, loanId, newPayment.Payment_Date, newPayment.Principal_Paid, newPayment.Interest_Paid).Scan(
		&paymentCreated.Payment_ID, &paymentCreated.Payment_Date, &paymentCreated.Principal_Paid, &paymentCreated.Interest_Paid); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": err})
		return
	}

	c.JSON(http.StatusCreated, paymentCreated)
}

func getAllPaymentsByLoanID(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	payments := []payment{}
	loanId := c.Param("loanId")

	queryString := `
		SELECT payment_id, payment_date, principal_paid, interest_paid
		FROM payment
		WHERE loan_id=$1
	`

	rows, err := db.Query(queryString, loanId)

	defer rows.Close()

	for rows.Next() {
		var p payment
		err := rows.Scan(&p.Payment_ID, &p.Payment_Date, &p.Principal_Paid, &p.Interest_Paid)
		if err != nil {
			log.Fatal(err)
		}
		payments = append(payments, p)
	}

	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}

	c.JSON(http.StatusOK, payments)
}

// case statement to return human readable errors
func retErrorStr(tag string) string {
	switch tag {
	case "required":
		return "This field is required."

	}

	return "Unknown Error"
}

func main() {
	db = connectDB()
	defer db.Close()

	router := gin.Default()

	//use json rather than struct field name
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})
	}

	//LOAN ROUTES
	router.GET("/loans", getAllLoans)
	router.POST("/loans", createNewLoan)

	router.GET("/loans/:loanId", getLoanByID)
	router.PATCH("/loans/:loanId", updateLoanByID)
	router.DELETE("/loans/:loanId", deleteLoanDataByID)

	//PAYMENT ROUTES
	router.POST("/loans/:loanId/payments", createNewPayment)
	router.GET("/loans/:loanId/payments", getAllPaymentsByLoanID)

	//Open Connection
	router.Run("localhost:8080")

}
