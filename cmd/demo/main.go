package main

import (
	"context"
	"flag"
	"fmt"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"kafka-utils/protos/gen/protos/cpu"
	"os"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sr"
)

const (
	latestVersion = -1
)

var (
	seedBrokers = flag.String("brokers", "localhost:9092", "comma delimited list of seed brokers")
	topic       = flag.String("topic", "cpu", "topic to produce to and consume from")
	registry    = flag.String("registry", "localhost:8085", "schema registry port to talk to")
)

func die(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

func maybeDie(err error, msg string, args ...interface{}) {
	if err != nil {
		die(msg, args...)
	}
}

func main() {

	ctx := context.Background()
	flag.Parse()

	// Ensure our schema is registered.
	rcl, err := sr.NewClient(sr.URLs(*registry))
	maybeDie(err, "unable to create schema registry client: %v", err)

	ss, err := rcl.SchemaByVersion(ctx, "protos.cpu", latestVersion, false)
	maybeDie(err, "failed to connect to schema registry")
	fmt.Printf("reusing schema subject %q version %d id %d\n", ss.Subject, ss.Version, ss.ID)

	// Setup our serializer / deserializer.
	maybeDie(err, "unable to parse avro schema: %v", err)
	var serde sr.Serde
	serde.Register(
		ss.ID,
		&cpu.CPU{},
		sr.EncodeFn(func(v interface{}) ([]byte, error) {
			fmt.Println("calling encode")
			s, err := proto.Marshal(v.(*cpu.CPU))
			if err != nil {
				fmt.Println("encoding error")
			}
			fmt.Println("Marshallnig done")
			return s, nil
		}),
		sr.DecodeFn(func(b []byte, v interface{}) error {
			return proto.Unmarshal(b, v.(*cpu.CPU))
		}),
		sr.GenerateFn(func() any {
			return new(cpu.CPU)
		}),
		sr.Index(0),
	)

	serde.Register(
		ss.ID,
		&cpu.RAM{},
		sr.EncodeFn(func(v interface{}) ([]byte, error) {
			fmt.Println("calling encode")
			s, err := proto.Marshal(v.(*cpu.RAM))
			if err != nil {
				fmt.Println("encoding error")
			}
			fmt.Println("Marshalling done")
			return s, nil
		}),
		sr.DecodeFn(func(b []byte, v interface{}) error {
			return proto.Unmarshal(b, v.(*cpu.RAM))
		}),
		sr.GenerateFn(func() any {
			return new(cpu.RAM)
		}),
		sr.Index(1),
	)

	// Loop producing & consuming.
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(strings.Split(*seedBrokers, ",")...),
		kgo.DefaultProduceTopic(*topic),
		kgo.ConsumeTopics(*topic),
		kgo.DialTimeout(time.Second*3),
		kgo.RetryTimeout(time.Second),
		//kgo.RequiredAcks(kgo.AllISRAcks()),
		//kgo.DisableAutoCommit(),
	)

	maybeDie(err, "unable to init kgo client: %v", err)

	timedCtx, cancelFn := context.WithTimeout(ctx, time.Second)
	defer cancelFn()
	maybeDie(cl.Ping(timedCtx), "unable to connect to kafka")

	for {
		fmt.Println("running demo")

		cl.Produce(
			context.Background(),
			&kgo.Record{
				Value: serde.MustEncode(
					&cpu.CPU{
						Brand:         "AKHIL",
						Name:          fmt.Sprintf("CPU-%d", time.Now().Unix()),
						NumberCores:   1,
						NumberThreads: 2,
						MinGhz:        3,
						MaxGhz:        4,
					}),
			},
			func(r *kgo.Record, err error) {
				maybeDie(err, "unable to produce: %v", err)
				fmt.Printf("Produced simple record, value bytes: %x\n", r.Value)
			},
		)

		cl.Produce(
			context.Background(),
			&kgo.Record{
				Value: serde.MustEncode(
					&cpu.RAM{
						Brand:  "AKHIL",
						Name:   fmt.Sprintf("RAM-%d", time.Now().Unix()),
						MinGhz: 3,
						MaxGhz: 4,
					}),
			},
			func(r *kgo.Record, err error) {
				maybeDie(err, "unable to produce: %v", err)
				fmt.Printf("Produced simple record, value bytes: %x\n", r.Value)
			},
		)

		fs := cl.PollFetches(context.Background())
		fs.EachRecord(func(r *kgo.Record) {
			res, err := serde.DecodeNew(r.Value)
			maybeDie(err, "unable to decode record value: %v")
			fmt.Printf("Consumed example: %+v, sleeping 1s\n", protojson.Format(res.(proto.Message)))
		})
		time.Sleep(time.Second)
	}
}
