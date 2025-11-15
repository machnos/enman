package main

import (
	"enman/internal/config"
	"enman/internal/controllers"
)

func main() {
	controllers.ProbePvController(&config.PvController{
		ConnectURL: "http://10.0.20.134",
		Type:       "http",
		Brand:      "Shelly",
	})
}
