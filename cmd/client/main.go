package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"otmane/pcbook/client"
	"otmane/pcbook/sample"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func testUploadImage(clientLaptop *client.LaptopClient) {
	laptop := sample.NewLaptop()
	clientLaptop.CreateLaptop(laptop)
	clientLaptop.UploadImage(laptop.GetId(), "tmp/wallpaper.jpg")
}

func testRateLaptop(clientLaptop *client.LaptopClient) {
	n := 3
	laptopIDS := make([]string, n)

	for i := 0; i < n; i++ {
		laptop := sample.NewLaptop()
		laptopIDS[i] = laptop.GetId()
		clientLaptop.CreateLaptop(laptop)
	}

	scores := make([]float64, n)
	for {
		fmt.Print("rate laptop (y/n)? ")
		var answer string
		fmt.Scan(&answer)

		if strings.ToLower(answer) != "y" {
			break
		}

		for i := 0; i < n; i++ {
			scores[i] = sample.RandomLaptopScore()
		}

		err := clientLaptop.RateLaptop(laptopIDS, scores)
		if err != nil {
			log.Fatal(err)
		}
	}
}

const (
	username        = "admin1"
	password        = "secret"
	refreshDuration = 3 * time.Second
)

func authMethods() map[string]bool {
	const laptopServicePath = "/pb.LaptopService/"

	return map[string]bool{
		laptopServicePath + "CreateLaptop": true,
		laptopServicePath + "UplaodImage":  true,
		laptopServicePath + "RateLaptop":   true,
	}
}

func main() {
	serverAddress := flag.String("address", "", "the server's address")
	flag.Parse()
	log.Printf("Dial server %s", *serverAddress)

	cc1, err := grpc.Dial(*serverAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("Cannot dial the server: ", err)
	}

	authClient := client.NewAuthClient(cc1, username, password)
	interceptor, err := client.NewAuthInterceptor(authClient, authMethods(), refreshDuration)
	if err != nil {
		panic(err)
	}

	cc2, err := grpc.Dial(
		*serverAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithUnaryInterceptor(interceptor.Unary()),
        grpc.WithStreamInterceptor(interceptor.Stream()),
	)
	if err != nil {
		panic(err)
	}

	laptopCLient := client.NewLaptopClient(cc2)
	testRateLaptop(laptopCLient)
}
