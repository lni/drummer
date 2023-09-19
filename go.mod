module github.com/lni/drummer/v3

require (
	github.com/cockroachdb/errors v1.9.0
	github.com/cockroachdb/pebble v0.0.0-20221207173255-0f086d933dac
	github.com/gogo/protobuf v1.3.2
	github.com/lni/dragonboat/v4 v4.0.0-20230917160253-d9f49378cd2d
	github.com/lni/goutils v1.3.1-0.20220604063047-388d67b4dbc4
	github.com/lni/vfs v0.2.1-0.20220616104132-8852fd867376
	golang.org/x/net v0.15.0
	google.golang.org/grpc v1.38.0
)

replace github.com/lni/dragonboat/v3 => /Users/lni/src/dragonboat

go 1.14
