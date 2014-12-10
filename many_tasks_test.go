package fezzik_test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"sync"
	"sync/atomic"

	"github.com/onsi/gomega/ghttp"

	"github.com/cloudfoundry/gunk/workpool"

	. "github.com/cloudfoundry-incubator/fezzik"
	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/runtime-schema/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func NewLightweightTask(guid string, addr string) receptor.TaskCreateRequest {
	return receptor.TaskCreateRequest{
		TaskGuid: guid,
		Domain:   domain,
		Stack:    stack,
		Action: &models.RunAction{
			Path: "bash",
			Args: []string{"-c", fmt.Sprintf("echo '%s' > /tmp/output", guid)},
		},
		CompletionCallbackURL: fmt.Sprintf("http://%s/done", addr),
		DiskMB:                64,
		MemoryMB:              64,
		ResultFile:            "/tmp/output",
	}
}

func TasksByDomainFetcher(domain string) func() ([]receptor.TaskResponse, error) {
	return func() ([]receptor.TaskResponse, error) {
		return client.TasksByDomain(domain)
	}
}

func safeWait(wg *sync.WaitGroup) chan struct{} {
	c := make(chan struct{})

	go func() {
		wg.Wait()
		close(c)
	}()

	return c
}

func NewGHTTPServer() (*ghttp.Server, string) {
	server := ghttp.NewUnstartedServer()
	l, err := net.Listen("tcp", "0.0.0.0:0")
	Ω(err).ShouldNot(HaveOccurred())
	server.HTTPTestServer.Listener = l
	server.HTTPTestServer.Start()

	re := regexp.MustCompile(`:(\d+)$`)
	port := re.FindStringSubmatch(server.URL())[1]
	Ω(port).ShouldNot(BeZero())

	//for bosh-lite only -- need something more sophisticated later.
	return server, fmt.Sprintf("%s:%s", publiclyAccessibleIP, port)
}

var _ = Describe("Running Many Tasks", func() {
	for _, factor := range []int{1, 5, 10, 20, 40} {
		factor := factor

		/*
			Commentary:

			Currently, this test shows a degradation in performance as `factor` increases.
			On Bosh-Lite I've traced this down to degrading Garden performance when many containers are created concurrently.
			This is unsuprising and is likely disk-io bound.  None of the degredation appears to be related to Diego's scheduling however.
		*/

		Context("when the tasks are lightweight (no downloads, no uploads)", func() {
			var workPool *workpool.WorkPool
			var tasks []receptor.TaskCreateRequest
			var taskReporter *TaskReporter
			var server *ghttp.Server

			BeforeEach(func() {
				var addr string

				workPool = workpool.NewWorkPool(500)
				server, addr = NewGHTTPServer()

				tasks = []receptor.TaskCreateRequest{}
				guid := NewGuid()
				for i := 0; i < factor*numCells; i++ {
					tasks = append(tasks, NewLightweightTask(fmt.Sprintf("%s-%d", guid, i), addr))
				}

				cells, err := client.Cells()
				Ω(err).ShouldNot(HaveOccurred())
				reportName := fmt.Sprintf("Running %d Tasks Across %d Cells", len(tasks), numCells)
				taskReporter = NewTaskReporter(reportName, len(tasks), cells)
			})

			AfterEach(func() {
				workPool.Stop()
				taskReporter.EmitSummary()
				taskReporter.Save()
			})

			It(fmt.Sprintf("should handle numCellx%d concurrent tasks", factor), func() {
				allCompleted := make(chan struct{})
				completionCounter := int64(0)
				server.RouteToHandler("POST", "/done", func(w http.ResponseWriter, req *http.Request) {
					defer func() {
						if atomic.AddInt64(&completionCounter, 1) >= int64(len(tasks)) {
							close(allCompleted)
						}
					}()
					var receivedTask receptor.TaskResponse
					json.NewDecoder(req.Body).Decode(&receivedTask)
					taskReporter.Completed(receivedTask)
				})

				wg := &sync.WaitGroup{}
				wg.Add(len(tasks))
				for _, task := range tasks {
					task := task
					workPool.Submit(func() {
						defer wg.Done()
						err := client.CreateTask(task)
						if err != nil {
							fmt.Println(err.Error())
							return
						}
						taskReporter.DidCreate(task.TaskGuid)
					})
				}

				Eventually(safeWait(wg), 240).Should(BeClosed())
				Eventually(allCompleted, 240).Should(BeClosed())
				Eventually(TasksByDomainFetcher(domain), 240).Should(BeEmpty())
			})
		})
	}
})
