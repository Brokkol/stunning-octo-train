package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type User struct{
	ID string `json:"id"`
	Name string `json:"name"`
	Email string `json:"email"`
}

type Stats struct{
	Status string `json:"status"`
	Uptime string `json:"uptime"`
	Request int `json:"requests"`
}

func main(){
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request){
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w,"OK")
	})

	http.HandleFunc("/api/user", func(w http.ResponseWriter, r *http.Request){
		if r.Method != http.MethodGet{
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
	

		user := User{
			ID:"1",
			Name:"Stas",
			Email:"email@test.com", // почему тут надо запятая
		}
		

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	})

	http.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request){
		if r.Method != http.MethodGet{
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		
		stats := Stats{
			Status:"running",
			Uptime: "7m",
			Request: 100,
		}

		w.Header().Set("Content-Type", "application/json/stats")
		json.NewEncoder(w).Encode(stats)

	})
	


	fmt.Println("Server is running in port 8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Printf("server failed: %s\n", err)
	}
}