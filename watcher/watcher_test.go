package watcher_test

import (
	"os"

	"github.com/cloudfoundry-incubator/runtime-schema/bbs/fake_bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	"github.com/cloudfoundry/gibson"
	"github.com/cloudfoundry/gosteno"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"

	"github.com/cloudfoundry-incubator/route-emitter/nats_emitter/fake_nats_emitter"
	"github.com/cloudfoundry-incubator/route-emitter/routing_table"
	"github.com/cloudfoundry-incubator/route-emitter/routing_table/fake_routing_table"
	. "github.com/cloudfoundry-incubator/route-emitter/watcher"
)

var _ = Describe("Watcher", func() {
	var (
		bbs                 *fake_bbs.FakeLRPRouterBBS
		table               *fake_routing_table.FakeRoutingTable
		emitter             *fake_nats_emitter.FakeNATSEmitter
		watcher             *Watcher
		process             ifrit.Process
		dummyMessagesToEmit routing_table.MessagesToEmit
	)

	BeforeEach(func() {
		bbs = fake_bbs.NewFakeLRPRouterBBS()
		table = &fake_routing_table.FakeRoutingTable{}
		emitter = &fake_nats_emitter.FakeNATSEmitter{}
		logger := gosteno.NewLogger("watcher")

		dummyContainer := routing_table.Container{Host: "1.1.1.1", Port: 11}
		dummyMessage := routing_table.RegistryMessageFor(dummyContainer, "foo.com", "bar.com")
		dummyMessagesToEmit = routing_table.MessagesToEmit{
			RegistrationMessages: []gibson.RegistryMessage{dummyMessage},
		}

		watcher = NewWatcher(bbs, table, emitter, logger)
		process = ifrit.Envoke(watcher)
	})

	AfterEach(func(done Done) {
		process.Signal(os.Interrupt)
		<-process.Wait()
		close(done)
	})

	Describe("when a desired LRP change arrives", func() {
		Context("when the change is a create/update (includes an after)", func() {
			BeforeEach(func() {
				desiredChange := models.DesiredLRPChange{
					Before: nil,
					After: &models.DesiredLRP{
						ProcessGuid: "pg",
						Routes:      []string{"route-1", "route-2"},
					},
				}

				table.SetRoutesReturns(dummyMessagesToEmit)

				bbs.DesiredLRPChangeChan <- desiredChange
			})

			It("should set the routes on the table", func() {
				Eventually(table.SetRoutesCallCount).Should(Equal(1))
				processGuid, routes := table.SetRoutesArgsForCall(0)
				Ω(processGuid).Should(Equal("pg"))
				Ω(routes).Should(Equal([]string{"route-1", "route-2"}))
			})

			It("should emit whatever the table tells it to emit", func() {
				Eventually(emitter.EmitCallCount).Should(Equal(1))
				Ω(emitter.EmitArgsForCall(0)).Should(Equal(dummyMessagesToEmit))
			})
		})

		Context("when the change is a delete (no after)", func() {
			BeforeEach(func() {
				desiredChange := models.DesiredLRPChange{
					Before: &models.DesiredLRP{
						ProcessGuid: "pg",
						Routes:      []string{"route-1"},
					},
					After: nil,
				}

				table.RemoveRoutesReturns(dummyMessagesToEmit)

				bbs.DesiredLRPChangeChan <- desiredChange
			})

			It("should remove the routes from the table", func() {
				Eventually(table.RemoveRoutesCallCount).Should(Equal(1))
				processGuid := table.RemoveRoutesArgsForCall(0)
				Ω(processGuid).Should(Equal("pg"))
			})

			It("should emit whatever the table tells it to emit", func() {
				Eventually(emitter.EmitCallCount).Should(Equal(1))
				Ω(emitter.EmitArgsForCall(0)).Should(Equal(dummyMessagesToEmit))
			})
		})
	})

	Describe("when an actual LRP change arrives", func() {
		Context("when the change is a create/update (includes an after)", func() {
			BeforeEach(func() {
				actualChange := models.ActualLRPChange{
					Before: nil,
					After: &models.ActualLRP{
						ProcessGuid: "pg",
						Host:        "1.1.1.1",
						State:       models.ActualLRPStateRunning,
						Ports: []models.PortMapping{
							{ContainerPort: 8080, HostPort: 11},
						},
					},
				}

				table.AddOrUpdateContainerReturns(dummyMessagesToEmit)

				bbs.ActualLRPChangeChan <- actualChange
			})

			It("should add/update the container on the table", func() {
				Eventually(table.AddOrUpdateContainerCallCount).Should(Equal(1))
				processGuid, container := table.AddOrUpdateContainerArgsForCall(0)
				Ω(processGuid).Should(Equal("pg"))
				Ω(container).Should(Equal(routing_table.Container{Host: "1.1.1.1", Port: 11}))
			})

			It("should emit whatever the table tells it to emit", func() {
				Eventually(emitter.EmitCallCount).Should(Equal(1))
				Ω(emitter.EmitArgsForCall(0)).Should(Equal(dummyMessagesToEmit))
			})
		})

		Context("when the change is a delete (no after)", func() {
			BeforeEach(func() {
				actualChange := models.ActualLRPChange{
					Before: &models.ActualLRP{
						ProcessGuid: "pg",
						Host:        "1.1.1.1",
						State:       models.ActualLRPStateRunning,
						Ports: []models.PortMapping{
							{ContainerPort: 8080, HostPort: 11},
						},
					},
					After: nil,
				}

				table.RemoveContainerReturns(dummyMessagesToEmit)

				bbs.ActualLRPChangeChan <- actualChange
			})

			It("should remove the container from the table", func() {
				Eventually(table.RemoveContainerCallCount).Should(Equal(1))
				processGuid, container := table.RemoveContainerArgsForCall(0)
				Ω(processGuid).Should(Equal("pg"))
				Ω(container).Should(Equal(routing_table.Container{Host: "1.1.1.1", Port: 11}))
			})

			It("should emit whatever the table tells it to emit", func() {
				Eventually(emitter.EmitCallCount).Should(Equal(1))
				Ω(emitter.EmitArgsForCall(0)).Should(Equal(dummyMessagesToEmit))
			})
		})
	})
})