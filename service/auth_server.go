package service

import (
	"context"
	"otmane/pcbook/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AuthServer is the server for authentication.
type AuthServer struct {
	pb.UnimplementedAuthServiceServer
	userStore  UserStore
	jwtManager JWTManager
}

// NewAuthServer returns a new Auth server.
func NewAuthServer(userStore UserStore, jwtManager JWTManager) *AuthServer {
    return &AuthServer{userStore: userStore, jwtManager: jwtManager}
}

func (server *AuthServer) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
    user, err := server.userStore.Find(req.GetUsername())
    if err != nil {
        return nil, status.Errorf(codes.Internal, "cannot find user: %v", err)
    }

    if user == nil || !user.IsCorrectPassword(req.GetPassword()) {
        return nil, status.Errorf(codes.NotFound, "incorrect username/password")
    }

    token, err := server.jwtManager.Generate(user)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "cannot generate access token")
    }

    return &pb.LoginResponse{
        AccessToken: token,
    }, nil
}
