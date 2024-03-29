package pgzip

import (
	"reflect"
	"testing"
)

func BenchmarkPGzipEncode(b *testing.B) {
	input := make([]byte, 1024)
	for i := 0; i < 1024; i++ {
		input[i] = 1
	}

	for i := 0; i < b.N; i++ {
		_, err := Encode(input)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPGzipDecode(b *testing.B) {
	input := make([]byte, 1024)
	for i := 0; i < 1024; i++ {
		input[i] = 1
	}

	got, _ := Encode(input)

	for i := 0; i < b.N; i++ {
		_, err := Decode(got)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestGzipDecode(t *testing.T) {
	type args struct {
		input []byte
	}

	decodeBytes := make([]byte, 1024)
	for i := 0; i < 1024; i++ {
		decodeBytes[i] = 1
	}

	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		// TODO: Add test cases.
		{"decode", args{input: []byte{31, 139, 8, 0, 0, 0, 0, 0, 0, 255, 98, 28, 5, 163, 96, 20, 140, 88, 0, 8, 0, 0, 255, 255, 29, 36, 130, 250, 0, 4, 0, 0}}, decodeBytes, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decode(tt.args.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("GzipDecode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GzipDecode() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGzipEncode(t *testing.T) {
	type args struct {
		input []byte
	}
	inputBytes := make([]byte, 1024)
	for i := 0; i < 1024; i++ {
		inputBytes[i] = 1
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		// TODO: Add test cases.
		{"encode", args{input: inputBytes}, []byte{31, 139, 8, 0, 0, 0, 0, 0, 0, 255, 98, 28, 5, 163, 96, 20, 140, 88, 0, 8, 0, 0, 255, 255, 29, 36, 130, 250, 0, 4, 0, 0}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Encode(tt.args.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("GzipEncode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GzipEncode() got = %v, want %v", got, tt.want)
			}
		})
	}
}
