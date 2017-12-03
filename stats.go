package stats

import (
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware"
	client "github.com/influxdata/influxdb/client/v2"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// InfluxOptions ...
type InfluxOptions struct {
	Measurement string
	Database    string
	Tags        map[string]string
	Fields      map[string]interface{}
}

// NewOpts returns a new options structure with the specified
// measurement, and database.
func NewOpts(measurement, database string) InfluxOptions {
	opts := InfluxOptions{}
	opts.Measurement = measurement
	opts.Database = database
	return opts
}

// NewInfluxClient creates a new InfluxDB client
func NewInfluxClient(address string) (client.Client, error) {
	addr := "127.0.0.1:8089"
	if address != "" {
		addr = address
	}

	influx, err := client.NewUDPClient(client.UDPConfig{Addr: addr})
	return influx, err
}

// WriteToInflux ....
// tags = things easy to index on
// fields - things that change a lot - ie. request time
func WriteToInflux(opts InfluxOptions, InfluxClient client.Client) error {
	bp, err := client.NewBatchPoints(client.BatchPointsConfig{Database: opts.Database})
	if err != nil {
		return err
	}
	pt, err := client.NewPoint(opts.Measurement, opts.Tags, opts.Fields, time.Now())
	if err != nil {
		return err
	}
	bp.AddPoint(pt)
	err = InfluxClient.Write(bp)
	if err != nil {
		return err
	}
	return nil
}

// UnaryServerInterceptor is a grpc middleware that logs latency
// to influx db.
func UnaryServerInterceptor(c client.Client, opts InfluxOptions) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		startTime := time.Now()
		resp, err := handler(ctx, req)
		latency := time.Now().Sub(startTime)

		tags := map[string]string{
			"method":     info.FullMethod,
			"error_code": string(grpc.Code(err)),
		}
		fields := map[string]interface{}{
			"latency": latency.Seconds() * 1000,
		}

		opts.Tags = tags
		opts.Fields = fields

		WriteToInflux(opts, c)

		return resp, err
	}
}

// StreamServerInterceptor returns a new streaming server
// interceptor that logs stats to influxdb.
func StreamServerInterceptor(c client.Client, opts InfluxOptions) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		startTime := time.Now()
		wrapped := grpc_middleware.WrapServerStream(stream)
		err := handler(srv, wrapped)

		latency := time.Now().Sub(startTime)

		tags := map[string]string{
			"method":     info.FullMethod,
			"error_code": string(grpc.Code(err)),
		}
		fields := map[string]interface{}{
			"latency": latency.Seconds() * 1000,
		}

		opts.Tags = tags
		opts.Fields = fields

		WriteToInflux(opts, c)

		return err
	}
}
