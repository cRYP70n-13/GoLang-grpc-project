package main

import (
	"bufio"
	"context"
	"flag"
	"io"
	"log"
	"os"
	"otmane/pcbook/pb"
	"otmane/pcbook/sample"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func createLatop(laptopClient pb.LaptopServiceClient, laptop *pb.Laptop) {
	req := &pb.CreateLaptopRequest{
		Laptop: laptop,
	}

	// Set the timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := laptopClient.CreateLaptop(ctx, req)
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

func searchLaptop(laptopClient pb.LaptopServiceClient, filter *pb.Filter) {
	log.Print("search filter: ", filter)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	searchLaptopRequest := &pb.SearchLaptopRequest{
		Filter: filter,
	}

	stream, err := laptopClient.SearchLaptop(ctx, searchLaptopRequest)
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

func uploadImage(laptopClient pb.LaptopServiceClient, laptopId string, imagePath string) {
    file, err := os.Open(imagePath)
    if err != nil {
        log.Fatal("cannot open image file", err)
    }
    defer file.Close()

    log.Printf("laptopID = %s", laptopId)

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    stream, err := laptopClient.UploadImage(ctx)
    if err != nil {
        log.Fatal("cannot upload the image", err)
    }

    req := &pb.UploadImageRequest{
        Data: &pb.UploadImageRequest_Info{
            Info: &pb.ImageInfo{
                LaptopId: laptopId,
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

func testCreateLaptop(laptopClient pb.LaptopServiceClient) {
    createLatop(laptopClient, sample.NewLaptop())
}

func testSearchLaptop(laptopClient pb.LaptopServiceClient) {
	for i := 0; i < 10; i++ {
		createLatop(laptopClient, sample.NewLaptop())
	}

	filter := &pb.Filter{
		MaxPriceUsd: 3000,
		MinCpuCors:  4,
		MinCpuGhz:   2.5,
		MinRam:      &pb.Memory{Value: 8, Unit: pb.Memory_GIGABYTE},
	}

    searchLaptop(laptopClient, filter)
}

func testUploadImage(laptopClient pb.LaptopServiceClient) {
    laptop := sample.NewLaptop()
    createLatop(laptopClient, laptop)
    uploadImage(laptopClient, laptop.GetId(), "tmp/wallpaper.jpg")
}

func main() {
	serverAddress := flag.String("address", "", "the server's address")
	flag.Parse()
	log.Printf("Dial server %s", *serverAddress)

	conn, err := grpc.Dial(*serverAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("Cannot dial the server: ", err)
	}

	laptopClient := pb.NewLaptopServiceClient(conn)
    testUploadImage(laptopClient)
}