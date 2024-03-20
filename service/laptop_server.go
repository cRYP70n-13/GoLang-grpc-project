package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"

	"otmane/pcbook/pb"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	MAX_ALLOWED_SIZE = 1 << 20 // 1MB
)

// LaptopServer is the server that provides laptop services
type LaptopServer struct {
	pb.UnimplementedLaptopServiceServer
	LaptopStore LaptopStore
	ImageStore  ImageStore
}

// NewLaptopServer creates a new laptop server instance and returns it
func NewLaptopServer(laptopStore LaptopStore, imageStore ImageStore) *LaptopServer {
	return &LaptopServer{LaptopStore: laptopStore, ImageStore: imageStore}
}

// CreateLaptop is a unary RPC to create a new laptop
func (server *LaptopServer) CreateLaptop(ctx context.Context, req *pb.CreateLaptopRequest) (*pb.CreateLaptopResponse, error) {
	laptop := req.GetLaptop()
	log.Printf("receive a create-laptop request with id: %s", laptop.Id)

	if len(laptop.Id) > 0 {
		// check if it's a valid UUID
		_, err := uuid.Parse(laptop.Id)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "laptop ID is not a valid UUID: %v", err)
		}
	} else {
		id, err := uuid.NewRandom()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "cannot generate a new laptop ID: %v", err)
		}
		laptop.Id = id.String()
	}

	// time.Sleep(6 * time.Second)

	if ctx.Err() == context.Canceled {
		log.Print("Client cancelled the request")
		return nil, status.Error(codes.Canceled, "RPC cancelled")
	}

	if ctx.Err() == context.DeadlineExceeded {
		log.Print("deadline exceeded for some reason")
		return nil, status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}

	// save the laptop to store
	err := server.LaptopStore.Save(laptop)
	if err != nil {
		code := codes.Internal
		if errors.Is(err, ErrAlreadyExists) {
			code = codes.AlreadyExists
		}

		return nil, status.Errorf(code, "cannot save laptop to the store: %v", err)
	}

	log.Printf("saved laptop with id: %s", laptop.Id)

	res := &pb.CreateLaptopResponse{
		Id: laptop.Id,
	}
	return res, nil
}

// SearchLaptop searches for a laptop with a filter and returns a stream of matching laptops
func (server *LaptopServer) SearchLaptop(req *pb.SearchLaptopRequest, stream pb.LaptopService_SearchLaptopServer) error {
	filter := req.GetFilter()
	log.Printf("receive a search-loop request with filter: %v", filter)

	err := server.LaptopStore.Search(stream.Context(), filter, func(laptop *pb.Laptop) error {
		res := &pb.SearchLaptopResponse{Laptop: laptop}
		err := stream.Send(res)
		if err != nil {
			return err
		}

		log.Printf("send laptop with id: %s", laptop.GetId())
		return nil
	})
	if err != nil {
		return status.Errorf(codes.Internal, "unexpected error: %v", err)
	}

	return nil
}

// UploadImage upload an image using streams to the server, and stores the info in DB
func (server *LaptopServer) UploadImage(stream pb.LaptopService_UploadImageServer) error {
	req, err := stream.Recv()
	if err != nil {
		return logError(status.Errorf(codes.Unknown, "cannot receive image info"))
	}

	laptopId := req.GetInfo().GetLaptopId()
	imageType := req.GetInfo().GetImageType()
	log.Printf("receive an upload-image request for laptop %s with image type %s: ", laptopId, imageType)

	laptop, err := server.LaptopStore.Find(laptopId)
	if err != nil {
		return logError(status.Errorf(codes.InvalidArgument, "cannot find laptop %s", laptopId))
	}
	if laptop == nil {
		return logError(status.Errorf(codes.InvalidArgument, "laptop %s doesn't exist", laptopId))
	}

	imageData := bytes.Buffer{}
	imageSize := 0

	for {
		if err := checkContextError(stream.Context()); err != nil {
			return err
		}
		log.Print("waiting to receive more data")

		req, err := stream.Recv()
		if err == io.EOF {
			log.Print("no more data")
			break
		}

		chunk := req.GetChunkData()
		size := len(chunk)

		log.Printf("received a chunk with size: %d", size)

		imageSize += size
		if imageSize > MAX_ALLOWED_SIZE {
			return logError(status.Errorf(codes.InvalidArgument, "image size is bigger then the allowed_size=%d", MAX_ALLOWED_SIZE))
		}

		_, err = imageData.Write(chunk)
		if err != nil {
			return logError(status.Errorf(codes.Internal, "cannot write chunk data: %v", err))
		}
	}

	imageID, err := server.ImageStore.Save(laptopId, imageType, imageData)
	if err != nil {
		return logError(status.Errorf(codes.Internal, "cannot save to the store %s", imageID))
	}

	res := &pb.UploadImageResponse{
		Id:   imageID,
		Size: uint32(imageSize),
	}

	err = stream.SendAndClose(res)
	if err != nil {
		return logError(status.Errorf(codes.Unknown, "cannot send the response to the client %v", err))
	}

	log.Printf("image %s successfully stored in our system", imageID)

	return nil
}

// RateLaptop is a bidirectional-streaming RPC that allows client to create a stream of laptops
// with a score, and returns a stream of average score for each of them.
// func (s *LaptopServer) RateLaptop(stream pb.LaptopService_RateLaptopServer) error {
// 	return nil
// }

func checkContextError(ctx context.Context) error {
	switch ctx.Err() {
	case context.Canceled:
		return logError(status.Errorf(codes.Canceled, "request is cancelled, server cancelling"))
	case context.DeadlineExceeded:
		return logError(status.Errorf(codes.Canceled, "deadline exceeded, server cancelling"))
	default:
		return nil
	}
}

func logError(err error) error {
	if err != nil {
		log.Print(err)
	}

	return err
}
