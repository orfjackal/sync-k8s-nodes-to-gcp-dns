// Copyright © 2018 Esko Luontola
// This software is released under the Apache License 2.0.
// The license text is at http://www.apache.org/licenses/LICENSE-2.0

package main

import (
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/dns/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
	"os"
	"time"
)

func main() {
	project := os.Getenv("GOOGLE_PROJECT")
	if project == "" {
		log.Fatal("Environment variable GOOGLE_PROJECT not set.")
	}

	googleApplicationCredentials := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if googleApplicationCredentials == "" {
		log.Fatal("Environment variable GOOGLE_APPLICATION_CREDENTIALS not set. " +
			"See https://cloud.google.com/docs/authentication/production for instructions.")
	}

	ctx := context.Background()

	client, err := google.DefaultClient(ctx, dns.CloudPlatformScope)
	if err != nil {
		log.Fatal(err)
	}

	dnsService, err := dns.New(client)
	if err != nil {
		log.Fatal(err)
	}

	records, err := getDnsRecords(project, ctx, dnsService)
	if err != nil {
		log.Fatal(err)
	}

	// TODO: parameterize
	records = filterDnsRecordsByName(records, "k8s-test1.luontola.fi.", "k8s-test2.luontola.fi.", "k8s-test3.luontola.fi.")

	spew.Dump(records)

	// TODO: get the k8s node IPs
	// TODO: check which DNS records have a different IP
	// TODO: update DNS records with new IPs

	// https://github.com/kubernetes/client-go/blob/master/examples/in-cluster-client-configuration/main.go

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	for {
		pods, err := clientset.CoreV1().Pods("").List(metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}
		fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))

		// Examples for error handling:
		// - Use helper functions like e.g. errors.IsNotFound()
		// - And/or cast to StatusError and use its properties like e.g. ErrStatus.Message
		_, err = clientset.CoreV1().Pods("default").Get("example-xxxxx", metav1.GetOptions{})
		if errors.IsNotFound(err) {
			fmt.Printf("Pod not found\n")
		} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
			fmt.Printf("Error getting pod %v\n", statusError.ErrStatus.Message)
		} else if err != nil {
			panic(err.Error())
		} else {
			fmt.Printf("Found pod\n")
		}

		time.Sleep(10 * time.Second)
	}
}

func filterDnsRecordsByName(records []*DnsRecord, names ...string) []*DnsRecord {
	if len(names) == 0 {
		return []*DnsRecord{}
	}
	var results []*DnsRecord
	for _, record := range records {
		if equalsAny(record.Name, names) {
			results = append(results, record)
		}
	}
	return results
}

func equalsAny(haystack string, needles []string) bool {
	for _, needle := range needles {
		if haystack == needle {
			return true
		}
	}
	return false
}

type DnsRecord struct {
	ManagedZone string
	*dns.ResourceRecordSet
}

func getDnsRecords(project string, ctx context.Context, dnsService *dns.Service) ([]*DnsRecord, error) {
	zones, err := getManagedZones(project, ctx, dnsService)
	if err != nil {
		return nil, err
	}
	var records []*DnsRecord
	for _, zone := range zones {
		rrsets, err := getResourceRecordSets(project, zone.Name, ctx, dnsService)
		if err != nil {
			return nil, err
		}
		for _, rrset := range rrsets {
			record := &DnsRecord{zone.Name, rrset}
			records = append(records, record)
		}

	}
	return records, nil
}

func getManagedZones(project string, ctx context.Context, dnsService *dns.Service) ([]*dns.ManagedZone, error) {
	var results []*dns.ManagedZone
	err := dnsService.ManagedZones.List(project).Pages(ctx, func(page *dns.ManagedZonesListResponse) error {
		for _, zone := range page.ManagedZones {
			results = append(results, zone)
		}
		return nil
	})
	return results, err
}

func getResourceRecordSets(project string, managedZone string, ctx context.Context, dnsService *dns.Service) ([]*dns.ResourceRecordSet, error) {
	var results []*dns.ResourceRecordSet
	req := dnsService.ResourceRecordSets.List(project, managedZone)
	err := req.Pages(ctx, func(page *dns.ResourceRecordSetsListResponse) error {
		for _, rrset := range page.Rrsets {
			results = append(results, rrset)
		}
		return nil
	})
	return results, err
}