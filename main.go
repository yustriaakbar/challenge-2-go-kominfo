package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
)

type Order struct {
	OrderID      int       `json:"orderId"`
	CustomerName string    `json:"customerName"`
	OrderedAt    time.Time `json:"orderedAt"`
	Items        []Item    `json:"items"`
}

type Item struct {
	LineItemID  int    `json:"lineItemId"`
	OrderID     int    `json:"orderId"`
	ItemCode    string `json:"itemCode"`
	Description string `json:"description"`
	Quantity    int    `json:"quantity"`
}

var db *sql.DB

func main() {
	// Connect to the MySQL database
	var err error
	db, err = sql.Open("mysql", "root:@tcp(127.0.0.1:3306)/challenge_2?parseTime=true")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Initialize the router
	router := mux.NewRouter()

	// Define API endpoints
	router.HandleFunc("/orders", createOrder).Methods("POST")
	router.HandleFunc("/orders", getOrders).Methods("GET")
	router.HandleFunc("/orders/{orderId}", updateOrder).Methods("PUT")
	router.HandleFunc("/orders/{orderId}", deleteOrder).Methods("DELETE")

	// Start the server
	log.Fatal(http.ListenAndServe(":9090", router))
}

func createOrder(w http.ResponseWriter, r *http.Request) {
	var order Order
	json.NewDecoder(r.Body).Decode(&order)

	// Insert into the database
	stmt, err := db.Prepare("INSERT INTO orders(customer_name, ordered_at) VALUES(?, ?)")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	res, err := stmt.Exec(order.CustomerName, order.OrderedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	orderID, err := res.LastInsertId()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, item := range order.Items {
		stmt, err := db.Prepare("INSERT INTO items(order_id, item_code, description, quantity) VALUES(?, ?, ?, ?)")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer stmt.Close()

		_, err = stmt.Exec(orderID, item.ItemCode, item.Description, item.Quantity)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "Order created successfully")
}

func getItemsByOrderID(orderID int) ([]Item, error) {
	rows, err := db.Query("SELECT * FROM items WHERE order_id = ?", orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var item Item
		err := rows.Scan(&item.LineItemID, &item.OrderID, &item.ItemCode, &item.Description, &item.Quantity)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}

func updateOrder(w http.ResponseWriter, r *http.Request) {
	// Extract order ID from URL
	params := mux.Vars(r)
	orderID, err := strconv.Atoi(params["orderId"])
	if err != nil {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	var order Order
	json.NewDecoder(r.Body).Decode(&order)

	// Update order details
	stmt, err := db.Prepare("UPDATE orders SET customer_name=?, ordered_at=? WHERE order_id=?")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(order.CustomerName, order.OrderedAt, orderID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Delete existing items for the order
	_, err = db.Exec("DELETE FROM items WHERE order_id=?", orderID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Insert updated items
	for _, item := range order.Items {
		stmt, err := db.Prepare("INSERT INTO items(order_id, item_code, description, quantity) VALUES(?, ?, ?, ?)")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer stmt.Close()

		_, err = stmt.Exec(orderID, item.ItemCode, item.Description, item.Quantity)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Order updated successfully")
}

func deleteOrder(w http.ResponseWriter, r *http.Request) {
	// Extract order ID from URL
	params := mux.Vars(r)
	orderID, err := strconv.Atoi(params["orderId"])
	if err != nil {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	// Delete associated items first
	_, err = db.Exec("DELETE FROM items WHERE order_id=?", orderID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Now delete the order
	_, err = db.Exec("DELETE FROM orders WHERE order_id=?", orderID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Order deleted successfully")
}

func getOrders(w http.ResponseWriter, r *http.Request) {
	// Query orders from the database
	rows, err := db.Query("SELECT * FROM orders")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var order Order
		err := rows.Scan(&order.OrderID, &order.CustomerName, &order.OrderedAt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Get items for the order
		items, err := getItemsByOrderID(order.OrderID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		order.Items = items

		orders = append(orders, order)
	}

	// Convert orders slice to JSON
	jsonOrders, err := json.Marshal(orders)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set content type and write JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonOrders)
}
