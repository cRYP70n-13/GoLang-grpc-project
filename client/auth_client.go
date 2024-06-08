package client

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"otmane/pcbook/pb"
)

// AuthClient is a client to call authentication RPC
type AuthClient struct {
	service  pb.AuthServiceClient
	username string
	password string
}

// NewAuthClient returns a new auth client.
func NewAuthClient(cc *grpc.ClientConn, username string, password string) *AuthClient {
	service := pb.NewAuthServiceClient(cc)
	return &AuthClient{service: service, username: username, password: password}
}

// Login logs a user in and returns its access token.
func (c *AuthClient) Login() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pb.LoginRequest{
		Username: c.username,
		Password: c.password,
	}

	res, err := c.service.Login(ctx, req)
	if err != nil {
		return "", err
	}

	return res.GetAccessToken(), nil
}
