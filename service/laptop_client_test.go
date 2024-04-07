package service_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	"otmane/pcbook/pb"
	"otmane/pcbook/sample"
	"otmane/pcbook/serializer"
	"otmane/pcbook/service"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestClientCreateLaptop(t *testing.T) {
	t.Parallel()

	laptopStore := service.NewInMemoryLaptopStore()
	serverAddress := startTestLaptopServer(t, laptopStore, nil, nil)
	laptopClient := newTestLaptopClient(t, serverAddress)

	laptop := sample.NewLaptop()
	expectedID := laptop.Id

	req := &pb.CreateLaptopRequest{
		Laptop: laptop,
	}

	res, err := laptopClient.CreateLaptop(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, res.Id, expectedID)

	// Check that the laptop is stored on the server
	other, err := laptopStore.Find(res.Id)
	require.NoError(t, err)
	require.NotNil(t, other)

	// Check that the stored laptop is the same one we sent
	requireSameLaptop(t, laptop, other)
}

// func TestClientRateLaptop(t *testing.T) {
//     t.Parallel()

//     laptopStore := service.NewInMemoryLaptopStore()
//     ratingStore := service.NewInMemoryRatingStore()

//     laptop := sample.NewLaptop()
//     err := laptopStore.Save(laptop)
//     require.NoError(t, err)

//     serverAddress := startTestLaptopServer(t, laptopStore, nil, ratingStore)
//     laptopCLient := newTestLaptopClient(t, serverAddress)

//     stream, err := laptopCLient.RateLaptop(context.Background())
//     require.NoError(t, err)

//     scores := []float64{8, 7.5, 10}
//     averages := []float64{8, 7.5, 10}
// }

func TestClientUploadImage(t *testing.T) {
	t.Parallel()

	testImageFolder := "../tmp"

	laptopStore := service.NewInMemoryLaptopStore()
	imageStore := service.NewDiskImageStore(testImageFolder)

	laptop := sample.NewLaptop()
	err := laptopStore.Save(laptop)
	require.NoError(t, err)

	serverAddress := startTestLaptopServer(t, laptopStore, imageStore, nil)
	laptopClient := newTestLaptopClient(t, serverAddress)

	imagePath := fmt.Sprintf("%s/wallpaper.jpg", testImageFolder)
	file, err := os.Open(imagePath)
	require.NoError(t, err)
	defer file.Close()

	stream, err := laptopClient.UploadImage(context.Background())
	require.NoError(t, err)

	imageType := filepath.Ext(imagePath)
	imageSize := 0

	req := &pb.UploadImageRequest{
		Data: &pb.UploadImageRequest_Info{
			Info: &pb.ImageInfo{
				LaptopId:  laptop.GetId(),
				ImageType: imageType,
			},
		},
	}
	err = stream.Send(req)
	require.NoError(t, err)

	reader := bufio.NewReader(file)
	buffer := make([]byte, 1024)

	for {
		n, err := reader.Read(buffer)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		imageSize += n

		req := &pb.UploadImageRequest{
			Data: &pb.UploadImageRequest_ChunkData{
				ChunkData: buffer[:n],
			},
		}

		err = stream.Send(req)
		require.NoError(t, err)
	}

	res, err := stream.CloseAndRecv()
	require.NoError(t, err)
	require.NotZero(t, res.GetId())
	require.EqualValues(t, imageSize, res.GetSize())

	savedImagePath := fmt.Sprintf("%s/%s%s", testImageFolder, res.GetId(), imageType)
	require.FileExists(t, savedImagePath)
	require.NoError(t, os.Remove(savedImagePath))
}

func TestClientSearchLaptop(t *testing.T) {
	t.Parallel()

	filter := &pb.Filter{
		MaxPriceUsd: 2000,
		MinCpuCors:  3,
		MinRam:      &pb.Memory{Value: 16, Unit: pb.Memory_GIGABYTE},
		MinCpuGhz:   3,
	}

	store := service.NewInMemoryLaptopStore()
	expectedIds := make(map[string]bool)

	for i := 0; i < 10; i++ {
		laptop := sample.NewLaptop()

		switch i {
		case 0:
			laptop.PriceUsd = 2500
		case 1:
			laptop.Ram = &pb.Memory{Value: 4, Unit: pb.Memory_GIGABYTE}
		case 2:
			laptop.PriceUsd = 1600
			laptop.Cpu.NumberCores = 4
			laptop.Cpu.MinGhz = 3.5
			laptop.Ram = &pb.Memory{Value: 32, Unit: pb.Memory_GIGABYTE}
			expectedIds[laptop.Id] = true
		case 3:
			laptop.PriceUsd = 1999
			laptop.Cpu.NumberCores = 6
			laptop.Cpu.MinGhz = 3.8
			laptop.Ram = &pb.Memory{Value: 32, Unit: pb.Memory_GIGABYTE}
			expectedIds[laptop.Id] = true
		case 4:
			laptop.Ram = &pb.Memory{Value: 8, Unit: pb.Memory_GIGABYTE}
		case 5:
			laptop.PriceUsd = 1999
			laptop.Cpu.NumberCores = 8
			laptop.Cpu.MinGhz = 3.8
			laptop.Ram = &pb.Memory{Value: 32, Unit: pb.Memory_GIGABYTE}
			expectedIds[laptop.Id] = true
		case 6:
			laptop.PriceUsd = 2000
			laptop.Cpu.NumberCores = 4
			laptop.Cpu.MinGhz = 3.8
			laptop.Ram = &pb.Memory{Value: 24, Unit: pb.Memory_GIGABYTE}
			expectedIds[laptop.Id] = true
		case 7:
			laptop.Ram = &pb.Memory{Value: 8, Unit: pb.Memory_GIGABYTE}
		case 8:
			laptop.Cpu.NumberCores = 3
			laptop.Ram = &pb.Memory{Value: 8, Unit: pb.Memory_GIGABYTE}
		case 9:
			laptop.Ram = &pb.Memory{Value: 12, Unit: pb.Memory_GIGABYTE}
		}

		err := store.Save(laptop)
		require.NoError(t, err)
	}

	serverAddress := startTestLaptopServer(t, store, nil, nil)
	laptopClient := newTestLaptopClient(t, serverAddress)

	req := &pb.SearchLaptopRequest{
		Filter: filter,
	}
	stream, err := laptopClient.SearchLaptop(context.Background(), req)
	require.NoError(t, err)

	found := 0
	for {
		res, err := stream.Recv()
		if err == io.EOF {
			break
		}

		require.NoError(t, err)
		require.Contains(t, expectedIds, res.Laptop.Id)
		found += 1
	}

	require.Equal(t, len(expectedIds), found)
}

func startTestLaptopServer(t *testing.T, store service.LaptopStore, imageStore service.ImageStore, ratingStore service.RatingStore) string {
	laptopServer := service.NewLaptopServer(store, imageStore, ratingStore)

	grpcServer := grpc.NewServer()
	pb.RegisterLaptopServiceServer(grpcServer, laptopServer)

	listener, err := net.Listen("tcp", ":0") // Any random available port
	require.NoError(t, err)

	go func() {
		err := grpcServer.Serve(listener) // Blocking call throw it in a separate goroutine
		if err != nil {
			t.Error("Something went KAKA while trying to serve gRPC server")
		}
	}()

	return listener.Addr().String()
}

func newTestLaptopClient(t *testing.T, address string) pb.LaptopServiceClient {
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	return pb.NewLaptopServiceClient(conn)
}

func requireSameLaptop(t *testing.T, laptop1, laptop2 *pb.Laptop) {
	json1, err := serializer.ProtobufToJSON(laptop1)
	require.NoError(t, err)

	json2, err := serializer.ProtobufToJSON(laptop2)
	require.NoError(t, err)

	require.Equal(t, json1, json2)
}
