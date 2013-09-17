package muster_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/daaku/go.muster"
)

type testClient struct {
	MaxBatchSize    int
	BatchTimeout    time.Duration
	PendingCapacity int
	Fire            func(items []string, notifier muster.Notifier)
	muster          muster.Client
}

func (c *testClient) Start() error {
	c.muster.MaxBatchSize = c.MaxBatchSize
	c.muster.BatchTimeout = c.BatchTimeout
	c.muster.PendingCapacity = c.PendingCapacity
	c.muster.BatchMaker = muster.BatchMakerFunc(
		func() muster.Batch { return &testBatch{testClient: c} })
	return c.muster.Start()
}

func (c *testClient) Stop() error {
	return c.muster.Stop()
}

func (c *testClient) Add(item string) {
	c.muster.Work <- item
}

type testBatch struct {
	testClient *testClient
	Items      []string
}

func (b *testBatch) Add(item interface{}) {
	b.Items = append(b.Items, item.(string))
}

func (b *testBatch) Fire(notifier muster.Notifier) {
	b.testClient.Fire(b.Items, notifier)
}

type fatal interface {
	Fatal(args ...interface{})
}

func errCall(t fatal, f func() error) {
	if err := f(); err != nil {
		t.Fatal(err)
	}
}

func expectFire(
	t *testing.T,
	finished chan struct{},
	expected [][]string,
) func(actual []string, notifier muster.Notifier) {

	return func(actual []string, notifier muster.Notifier) {
		defer notifier.Done()
		defer close(finished)
		for _, batch := range expected {
			if !reflect.DeepEqual(actual, batch) {
				t.Fatalf("expected %v\nactual %v", batch, actual)
			}
		}
	}
}

func addExpected(c *testClient, expected [][]string) {
	for _, b := range expected {
		for _, v := range b {
			c.Add(v)
		}
	}
}

func TestMaxBatch(t *testing.T) {
	t.Parallel()
	expected := [][]string{{"milk", "yogurt", "butter"}}
	finished := make(chan struct{})
	c := &testClient{
		MaxBatchSize:    len(expected[0][0]),
		BatchTimeout:    20 * time.Millisecond,
		Fire:            expectFire(t, finished, expected),
		PendingCapacity: 100,
	}
	errCall(t, c.Start)
	addExpected(c, expected)
	<-finished
}

func TestBatchTimeout(t *testing.T) {
	t.Parallel()
	expected := [][]string{{"milk", "yogurt"}}
	finished := make(chan struct{})
	c := &testClient{
		MaxBatchSize:    3,
		BatchTimeout:    20 * time.Millisecond,
		Fire:            expectFire(t, finished, expected),
		PendingCapacity: 100,
	}
	errCall(t, c.Start)
	addExpected(c, expected)
	time.Sleep(30 * time.Millisecond)
	<-finished
}

func TestStop(t *testing.T) {
	t.Parallel()
	expected := [][]string{{"milk", "yogurt"}}
	finished := make(chan struct{})
	c := &testClient{
		MaxBatchSize:    3,
		BatchTimeout:    time.Hour,
		Fire:            expectFire(t, finished, expected),
		PendingCapacity: 100,
	}
	errCall(t, c.Start)
	addExpected(c, expected)
	errCall(t, c.Stop)
	<-finished
}

func TestZeroMaxBatchSize(t *testing.T) {
	t.Parallel()
	expected := [][]string{{"milk", "yogurt"}}
	finished := make(chan struct{})
	c := &testClient{
		BatchTimeout:    20 * time.Millisecond,
		Fire:            expectFire(t, finished, expected),
		PendingCapacity: 100,
	}
	errCall(t, c.Start)
	addExpected(c, expected)
	time.Sleep(30 * time.Millisecond)
	<-finished
}

func TestZeroBatchTimeout(t *testing.T) {
	t.Parallel()
	expected := [][]string{{"milk", "yogurt"}}
	finished := make(chan struct{})
	c := &testClient{
		MaxBatchSize:    3,
		Fire:            expectFire(t, finished, expected),
		PendingCapacity: 100,
	}
	errCall(t, c.Start)
	addExpected(c, expected)
	time.Sleep(30 * time.Millisecond)
	select {
	case <-finished:
		t.Fatal("should not be finished yet")
	default:
	}
	errCall(t, c.Stop)
	<-finished
}

func TestZeroBoth(t *testing.T) {
	t.Parallel()
	c := &testClient{}
	if c.Start() == nil {
		t.Fatal("was expecting error")
	}
}

func BenchmarkFlow(b *testing.B) {
	c := &testClient{
		MaxBatchSize:    3,
		BatchTimeout:    time.Hour,
		PendingCapacity: 100,
		Fire: func(actual []string, notifier muster.Notifier) {
			notifier.Done()
		},
	}
	errCall(b, c.Start)
	for i := 0; i < b.N; i++ {
		c.Add("42")
	}
	errCall(b, c.Stop)
}
