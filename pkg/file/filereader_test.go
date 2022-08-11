package file

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

	assert.EqualError(t, actual, "reading <../../api/testdata/enoent.yaml>: open ../../api/testdata/enoent.yaml: no such file or directory")
}

func TestFileReaderInitializeBogus(t *testing.T) {
	sut := NewFileReader("../../api/testdata/bogus.yaml")

	actual := sut.Initialize(&testObject{})

	assert.EqualError(t, actual, "error parsing <../../api/testdata/bogus.yaml>:\nbogus\n")
}

func TestFileReaderInitializeGood(t *testing.T) {
	sut := NewFileReader("../../api/testdata/multiple.yaml")

	actual := sut.Initialize(&testObject{})

	assert.Nil(t, actual)
}

func TestFileReaderParseMissing(t *testing.T) {
	config := testObject{}
	sut := NewFileReader("../../api/testdata/missing.yaml")
	initError := sut.Initialize(&config)
	assert.Nil(t, initError)

	actual := sut.Parse(&config)

	assert.EqualError(t, actual, "could not find <testObject> in cluster configuration ../../api/testdata/missing.yaml")
}

func TestFileReaderParseGood(t *testing.T) {
	config := &testObject{}
	sut := NewFileReader("../../api/testdata/multiple.yaml")
	initError := sut.Initialize(config)
	assert.Nil(t, initError)

	actual := sut.Parse(config)

	assert.Nil(t, actual)
}

// Ensure that the api.FileReader can correctly unmarshal the API types we've defined.
func TestFileReaderOnApiDatatypes(t *testing.T) {
	_, err := GivenBundleController("../../api/testdata/packagebundlecontroller.yaml")
	assert.NoError(t, err)

	_, err = GivenPackageBundle("../../api/testdata/bundle_two.yaml")
	assert.NoError(t, err)

	_, err = GivenPackageBundle("../../api/testdata/bundle_one.yaml")
	assert.NoError(t, err)

	_, err = GivenPackage("../../api/testdata/test.yaml")
	assert.NoError(t, err)
}
