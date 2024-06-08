package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"otmane/pcbook/pb"
	"otmane/pcbook/service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	secretKey     = "supersecret"
	tokenDuration = 15 * time.Minute
)

func seedUsers(userStore service.UserStore) error {
	err := createUser(userStore, "admin1", "secret", "admin")
	if err != nil {
		return err
	}

	return createUser(userStore, "admin2", "secret", "user")
}

func createUser(userStore service.UserStore, username, password, role string) error {
	user, err := service.NewUser(username, password, role)
	if err != nil {
		return err
	}

	return userStore.Save(user)
}

func accessibleRoles() map[string][]string {
	const laptopServicePath = "/pb.LaptopService/"

	return map[string][]string{
		laptopServicePath + "CreateLaptop": {"admin"},
		laptopServicePath + "UploadImage":  {"admin"},
		laptopServicePath + "RateLaptop":   {"admin", "user"},
	}
}

func main() {
	port := flag.Int("port", 5050, "the server port")
	enableTls := flag.Bool("tls", false, "enable SSL/TLS")
	flag.Parse()
	log.Printf("Start server on port: %d, TLS = %t", *port, *enableTls)

	laptopStore := service.NewInMemoryLaptopStore()
	imageStore := service.NewDiskImageStore("img")
	ratingStore := service.NewInMemoryRatingStore()

	userStore := service.NewInMemoryUserStore()
	jwtManager := service.NewJWTManager(secretKey, tokenDuration)

	if err := seedUsers(userStore); err != nil {
		panic(err)
	}

	interceptor := service.NewAuthInterceptor(jwtManager, accessibleRoles())
	serverOptions := []grpc.ServerOption{
		grpc.UnaryInterceptor(interceptor.Unary()),
		grpc.StreamInterceptor(interceptor.Stream()),
	}

	grpcServer := grpc.NewServer(serverOptions...)

	authServer := service.NewAuthServer(userStore, *jwtManager)

	laptopServer := service.NewLaptopServer(laptopStore, imageStore, ratingStore)

	pb.RegisterLaptopServiceServer(grpcServer, laptopServer)
	pb.RegisterAuthServiceServer(grpcServer, authServer)

	address := fmt.Sprintf("0.0.0.0:%d", *port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal("Cannot start server: ", err)
	}

	reflection.Register(grpcServer)

	err = grpcServer.Serve(listener)
	if err != nil {
		log.Fatal("Cannot start server: ", err)
	}
}
