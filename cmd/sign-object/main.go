package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/yaml"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/signable"
)

const (
	defaultKey    = "private.ec.key"
	defaultOutput = "-"
)

func main() {
	var keyFile string
	var outputFile string

	rootCmd := &cobra.Command{
		Use:   "sign-object -s filename [-o object.signed.yaml] [object.yaml]",
		Short: "Sign a kubernetes object file",
		Long: `Sign a kubernetes object YAML file.

The signing key is expected to be PEM-encoded.

If no bundle file is supplied, reads from stdin.

If no output file is specified, writes to stdout.
`,
		Args: cobra.MaximumNArgs(1), // a kubernetes object file for input
		RunE: sign,
	}
	rootCmd.Flags().StringVarP(&keyFile, "signing-key", "s", defaultKey,
		"private key filename for signing the package bundle")
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", defaultOutput,
		"output bundle filename")
	if err := rootCmd.MarkFlagRequired("signing-key"); err != nil {
		log.Fatal(err)
	}
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

var DefaultGroup = api.GroupVersion.Group
var DefaultVersion = api.GroupVersion.Version

func sign(cmd *cobra.Command, args []string) error {
	var err error

	objectFile := os.Stdin
	if len(args) > 0 && args[0] != "-" {
		objectFile, err = os.Open(args[0])
		if err != nil {
			return fmt.Errorf("opening object file %q: %w", args[0], err)
		}
		defer objectFile.Close()
	}

	gvk := schema.GroupVersionKind{}
	scheme := runtime.NewScheme()
	utilruntime.Must(api.AddToScheme(scheme))
	buf := &bytes.Buffer{}
	_, err = io.Copy(buf, objectFile)
	if err != nil {
		return fmt.Errorf("reading object: %w", err)
	}

	err = yaml.Unmarshal(buf.Bytes(), &gvk)
	if err != nil {
		return fmt.Errorf("unmarshaling object's gvk: %w", err)
	}
	if gvk.Group == "" {
		gvk.Group = DefaultGroup
	}
	if gvk.Version == "" {
		gvk.Version = DefaultVersion
	}
	object, err := scheme.New(gvk)
	if err != nil {
		return fmt.Errorf("finding gvk in schema: %w", err)
	}

	err = yaml.Unmarshal(buf.Bytes(), object)
	if err != nil {
		return fmt.Errorf("unmarshaling object: %w", err)
	}

	pemKeyFilename := cmd.Flag("signing-key").Value.String()
	pemPrivKey, err := os.ReadFile(pemKeyFilename)
	if err != nil {
		return fmt.Errorf("reading signing key file %q: %w", pemKeyFilename, err)
	}

	s := signable.New(object.(signable.Object))
	objectSigned, err := s.SignedYAML(pemPrivKey)
	if err != nil {
		return err
	}

	signedObjectFile := os.Stdout
	outputFilename := cmd.Flag("output").Value.String()
	if outputFilename != "" && outputFilename != "-" {
		if signedObjectFile, err = os.Open(outputFilename); err != nil {
			return fmt.Errorf("opening object file %q: %w",
				outputFilename, err)
		}
		defer signedObjectFile.Close()
	}

	fmt.Fprintln(signedObjectFile, string(objectSigned))

	return nil
}
