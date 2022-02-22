package api

import (
	"bytes"
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

const (
	yamlSeparator = "\n---\n"
)

// KindAccessor exposes the Kind field for Cluster type.
//
// The FileReader will compare the Kind field (accessed via MetaKind()) with the
// result of ExpectedKind() to ensure that the data we unmarshaled was meant for
// a struct of the correct type. That is, it prevents us from unmarshaling bytes
// meant for a Foo struct into a Bar struct.
type KindAccessor interface {
	// MetaKind is the kind actually read when unmarshaling from a file.
	MetaKind() string

	// ExpectedKind is the kind we expect to read while unmarshaling.
	ExpectedKind() string
}

type FileReader struct {
	fileName       string
	clusterConfigs map[string][]byte
}

func NewFileReader(fileName string) *FileReader {
	return &FileReader{
		fileName:       fileName,
		clusterConfigs: map[string][]byte{},
	}
}

// sliceYAML returns a slice of YAML documents from a file.
func (reader *FileReader) sliceYAML() ([][]byte, error) {
	content, err := os.ReadFile(reader.fileName)
	if err != nil {
		return nil, err
	}
	return bytes.Split(content, []byte(yamlSeparator)), nil
}

func (reader *FileReader) Initialize(clusterConfig KindAccessor) error {
	yamls, err := reader.sliceYAML()
	if err != nil {
		return fmt.Errorf("reading <%s>: %v", reader.fileName, err)
	}

	for _, config := range yamls {
		if err = yaml.Unmarshal(config, clusterConfig); err != nil {
			return fmt.Errorf("error parsing <%s>:\n%s", reader.fileName, config)
		}
		reader.clusterConfigs[clusterConfig.MetaKind()] = config
	}
	return nil
}

func (reader *FileReader) Parse(clusterConfig KindAccessor) error {
	if val, ok := reader.clusterConfigs[clusterConfig.ExpectedKind()]; ok {
		return ParseByteSlice(val, clusterConfig)
	}
	return fmt.Errorf("could not find <%s> in cluster configuration %s", clusterConfig.ExpectedKind(), reader.fileName)
}

func ParseByteSlice(data []byte, clusterConfig KindAccessor) error {
	return yaml.UnmarshalStrict(data, clusterConfig)
}
