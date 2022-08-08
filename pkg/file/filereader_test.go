package file

import (
	"errors"
	"testing"
)

type testObject struct {
	Kind string `json:"kind"`
	Foo  int    `json:"foo"`
}

func (o *testObject) ExpectedKind() string {
	return "testObject"
}

func (o *testObject) MetaKind() string {
	return o.Kind
}

func TestFileReaderInitializeEnoent(t *testing.T) {
	sut := NewFileReader("../../api/testdata/enoent.yaml")

	actual := sut.Initialize(&testObject{})

	expected := errors.New("reading <../../api/testdata/enoent.yaml>: open ../../api/testdata/enoent.yaml: no such file or directory")
	if actual == nil || actual.Error() != expected.Error() {
		t.Errorf("expected <%v> actual <%v>", expected, actual)
	}
}

func TestFileReaderInitializeBogus(t *testing.T) {
	sut := NewFileReader("../../api/testdata/bogus.yaml")

	actual := sut.Initialize(&testObject{})

	expected := errors.New("error parsing <../../api/testdata/bogus.yaml>:\nbogus\n")
	if actual == nil || actual.Error() != expected.Error() {
		t.Errorf("expected <%v> actual <%v>", expected, actual)
	}
}

func TestFileReaderInitializeGood(t *testing.T) {
	sut := NewFileReader("../../api/testdata/multiple.yaml")

	actual := sut.Initialize(&testObject{})

	if actual != nil {
		t.Errorf("expected <%v> actual <%v>", nil, actual)
	}
}

func TestFileReaderParseMissing(t *testing.T) {
	config := testObject{}
	sut := NewFileReader("../../api/testdata/missing.yaml")
	initError := sut.Initialize(&config)
	if initError != nil {
		t.Errorf("Initialize expected <nil> actual <%v>", initError)
	}

	actual := sut.Parse(&config)

	expected := errors.New("could not find <testObject> in cluster configuration ../../api/testdata/missing.yaml")
	if actual == nil || actual.Error() != expected.Error() {
		t.Errorf("expected <%v> actual <%v>", expected, actual)
	}
}

func TestFileReaderParseGood(t *testing.T) {
	config := &testObject{}
	sut := NewFileReader("../../api/testdata/multiple.yaml")
	initError := sut.Initialize(config)
	if initError != nil {
		t.Errorf("Initialize expected <nil> actual <%v>", initError)
	}

	actual := sut.Parse(config)

	if actual != nil {
		t.Errorf("expected <%v> actual <%v>", nil, actual)
	}
}

// Ensure that the api.FileReader can correctly unmarshal the API types we've defined.
func TestFileReaderOnApiDatatypes(t *testing.T) {
	_, err := GivenBundleController("../../api/testdata/packagebundlecontroller.yaml")
	if err != nil {
		t.Errorf("expected <%v> actual <%v>", nil, err)
	}

	_, err = GivenPackageBundle("../../api/testdata/bundle_two.yaml")
	if err != nil {
		t.Errorf("expected <%v> actual <%v>", nil, err)
	}

	_, err = GivenPackageBundle("../../api/testdata/bundle_one.yaml")
	if err != nil {
		t.Errorf("expected <%v> actual <%v>", nil, err)
	}

	_, err = GivenPackage("../../api/testdata/test.yaml")
	if err != nil {
		t.Errorf("expected <%v> actual <%v>", nil, err)
	}
}
