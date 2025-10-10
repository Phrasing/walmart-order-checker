package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
)

func generateCSV(orders map[string]*Order, path string) {
	file, err := os.Create(path)
	if err != nil {
		log.Fatalf("Failed to create CSV file: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"Order ID", "Order Date", "Order Total", "Item Name", "Quantity"})
	for _, order := range orders {
		if order.Status != "canceled" {
			for _, item := range order.Items {
				writer.Write([]string{formatOrderID(order.ID), order.OrderDate, order.Total, item.Name, fmt.Sprintf("%d", item.Quantity)})
			}
		}
	}
	fmt.Printf("\nNon-canceled orders have been written to %s\n", path)
}

func generateShippedCSV(shippedOrders []*ShippedOrder, path string) {
	file, err := os.Create(path)
	if err != nil {
		log.Fatalf("Failed to create CSV file: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"Order ID", "Carrier", "Tracking #", "Estimated Arrival"})
	for _, order := range shippedOrders {
		writer.Write([]string{formatOrderID(order.ID), order.Carrier, order.TrackingNumber, order.EstimatedArrival})
	}
	fmt.Printf("\nShipped orders have been written to %s\n", path)
}
