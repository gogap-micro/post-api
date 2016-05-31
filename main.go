package main

import (
	"github.com/gogap-micro/post-api/api"
)

func main() {

	api, _ := api.NewPostAPI(
		api.Address(":8088"),
		api.CORS(api.CORSOptions{
			AllowOrigins:     []string{"*"},
			AllowMethods:     []string{"POST", "OPTIONS"},
			AllowCredentials: true,
		}),
		api.ResponseHeader("Server", "post-api"),
		api.Path("/api"),
	)

	api.Run()
}
