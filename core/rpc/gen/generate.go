package gen

//go:generate go tool buf generate --config ../../../buf.yaml --template ../../../buf.gen.yaml --output ../../.. ../../..

// The REST layer serializes generated messages with encoding/json; stripping
// omitempty keeps zero-valued fields in responses, matching the API contract.
//go:generate sh -c "sed -i 's/,omitempty\"/\"/g' v1/*.pb.go"
