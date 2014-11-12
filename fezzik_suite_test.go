package fezzik_test

import (
	"flag"
	"log"

	"github.com/cloudfoundry-incubator/receptor"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var receptorAddress, receptorUsername, receptorPassword, publiclyAccessibleIP string
var numCells int

var client receptor.Client
var domain, stack string

func init() {
	flag.StringVar(&receptorAddress, "receptor-address", "receptor.10.244.0.34.xip.io", "http address for the receptor (required)")
	flag.StringVar(&receptorUsername, "receptor-username", "", "receptor username")
	flag.StringVar(&receptorUsername, "receptor-password", "", "receptor password")
	flag.StringVar(&publiclyAccessibleIP, "publicly-accessible-ip", "192.168.220.1", "a publicly accessible IP for the host the test is running on (necssary to run a local server that containers can phone home to)")
	flag.IntVar(&numCells, "num-cells", 10, "number of cells")
	flag.Parse()

	if receptorAddress == "" {
		log.Fatal("i need a receptor-address to talk to Diego...")
	}
}

func TestFezzik(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Fezzik Suite")
}

var _ = BeforeSuite(func() {
	client = receptor.NewClient(receptorAddress, receptorUsername, receptorPassword)
	domain = "fezzik"
	stack = "lucid64"
})