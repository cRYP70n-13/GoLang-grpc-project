package client

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"otmane/pcbook/pb"
)

type LaptopClient struct {
    service pb.LaptopServiceClient
}

func NewLaptopClient(cc *grpc.ClientConn) *LaptopClient {
    service := pb.NewLaptopServiceClient(cc)
    return &LaptopClient{
        service: service,
    }
}

func (l *LaptopClient) CreateLaptop(laptop *pb.Laptop) {
	req := &pb.CreateLaptopRequest{
		Laptop: laptop,
	}

	// Set the timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := l.service.CreateLaptop(ctx, req)
	if err != nil {
		sts, ok := status.FromError(err)
		if ok && sts.Code() == codes.AlreadyExists {
			log.Print("Laptop already exists")
		} else {
			log.Fatal("Cannot create laptop: ", err)
		}

		return
	}

	log.Printf("created laptop with ID: %v", res.Id)
}

func (l *LaptopClient) SearchLaptop(filter *pb.Filter) {
	log.Print("search filter: ", filter)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	searchLaptopRequest := &pb.SearchLaptopRequest{
		Filter: filter,
	}

	stream, err := l.service.SearchLaptop(ctx, searchLaptopRequest)
	if err != nil {
		log.Fatal("cannot search for the laptop: ", err)
	}

	for {
		res, err := stream.Recv()
		if err == io.EOF {
			// Done with receiving from the stream
			return
		}
		if err != nil {
			log.Fatal("cannot receive the response: ", err)
		}

		laptop := res.Laptop
		log.Print("- Found: ", laptop.GetId())
		log.Print("     + BRAND: ", laptop.GetBrand())
		log.Print("     + RAM: ", laptop.GetRam())
		log.Print("     + cpu cores: ", laptop.GetCpu().GetNumberCores())
		log.Print("     + cpu min ghz: ", laptop.GetCpu().GetMinGhz())
	}
}

func (l *LaptopClient) UploadImage(laptopId string, imagePath string) {
	file, err := os.Open(imagePath)
	if err != nil {
		log.Fatal("cannot open image file", err)
	}
	defer file.Close()

	log.Printf("laptopID = %s", laptopId)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := l.service.UploadImage(ctx)
	if err != nil {
		log.Fatal("cannot upload the image", err)
	}

	req := &pb.UploadImageRequest{
		Data: &pb.UploadImageRequest_Info{
			Info: &pb.ImageInfo{
				LaptopId:  laptopId,
				ImageType: filepath.Ext(imagePath),
			},
		},
	}
	err = stream.Send(req)
	if err != nil {
		log.Fatal("cannot stream the file to the server", err)
	}

	reader := bufio.NewReader(file)
	buffer := make([]byte, 1024)

	for {
		n, err := reader.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal("cannot read chunk to buffer: ", err)
		}

		req := &pb.UploadImageRequest{
			Data: &pb.UploadImageRequest_ChunkData{
				ChunkData: buffer[:n],
			},
		}

		err = stream.Send(req)
		if err != nil {
			log.Fatal("cannot stream the file to the server", err)
		}
	}

	res, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatal("cannot receive from the server", err)
	}
	log.Printf("image is successfully uploaded with ID: %s and size %d", res.GetId(), res.GetSize())
}

func (l *LaptopClient) RateLaptop(laptopIDs []string, scores []float64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := l.service.RateLaptop(ctx)
	if err != nil {
		return fmt.Errorf("cannot rate laptop: %v", err)
	}

	waitResponse := make(chan error)

	// Go routines to receive responses
	go func() {
		for {
			res, err := stream.Recv()
			if err == io.EOF {
				log.Print("nothing else to read")
				waitResponse <- nil
				return
			}
			if err != nil {
				waitResponse <- fmt.Errorf("cannot receive stream response: %v", err)
				return
			}

			log.Print("received response", res)
		}
	}()

	// Send the requests.
	for i, laptopId := range laptopIDs {
		req := &pb.RateLaptopRequest{
			LaptopId: laptopId,
			Score:    scores[i],
		}

		err := stream.Send(req)
		if err != nil {
			return fmt.Errorf("cannot send request to the server: %v - %v", err, stream.RecvMsg(nil))
		}

		log.Print("sent request: ", req)
	}

	err = stream.CloseSend()
	if err != nil {
		return fmt.Errorf("cannot close send: %v", err)
	}

	err = <-waitResponse
	return err
}
