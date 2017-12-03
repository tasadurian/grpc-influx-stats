# grpc-influx-stats
a stats package for grpc and influxdb

## Usage

```
func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	c, err := stats.NewInfluxClient("127.0.0.1:8089")
	if err != nil {
		log.Fatal(err)
	}

	infOpts := stats.NewOpts("my_measurment", "my_database")

	s := grpc.NewServer(grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
		stats.UnaryServerInterceptor(c, infOpts),
	)))

	pb.RegisterGreeterServer(s, &server{})
	// Register reflection service on gRPC server.
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
```