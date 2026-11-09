package quote

import "testing"

func TestUploadRequestValidation(t *testing.T) {
	valid := UploadRequest{Filename: "model.stl", ContentType: "model/stl", Size: 1024, Checksum: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if _, err := valid.Validate(); err != nil {
		t.Fatalf("valid upload: %v", err)
	}
	cases := []UploadRequest{{Filename: "x.exe", ContentType: "model/stl", Size: 1, Checksum: valid.Checksum}, {Filename: "x.stl", ContentType: "application/zip", Size: 1, Checksum: valid.Checksum}, {Filename: "x.stl", ContentType: "model/stl", Size: MaxFileSize + 1, Checksum: valid.Checksum}, {Filename: "x.stl", ContentType: "model/stl", Size: 1, Checksum: "bad"}}
	for _, test := range cases {
		if _, err := test.Validate(); err == nil {
			t.Errorf("expected invalid: %+v", test)
		}
	}
}
func TestCreateRequestValidation(t *testing.T) {
	request := CreateRequest{Name: "Cliente", Email: "cliente@example.com", MaterialID: "m", ColorID: "c", Quantity: 1, UploadID: "u", UploadToken: "t"}
	if err := request.Validate(); err != nil {
		t.Fatalf("valid request: %v", err)
	}
	request.Quantity = 0
	if err := request.Validate(); err == nil {
		t.Fatal("expected invalid quantity")
	}
}
