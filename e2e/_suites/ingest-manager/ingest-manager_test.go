package main

import (
	"os"
	"path"
	"time"

	"github.com/cucumber/godog"
	"github.com/cucumber/messages-go/v10"
	"github.com/elastic/e2e-testing/cli/config"
	"github.com/elastic/e2e-testing/cli/services"
	"github.com/elastic/e2e-testing/e2e"
	log "github.com/sirupsen/logrus"
)

// stackVersion is the version of the stack to use
// It can be overriden by OP_STACK_VERSION env var
var stackVersion = "7.7.0"

// profileEnv is the environment to be applied to any execution
// affecting the runtime dependencies (or profile)
var profileEnv map[string]string

// All URLs running on localhost as Kibana is expected to be exposed there
const kibanaBaseURL = "http://localhost:5601"

func init() {
	config.Init()

	stackVersion = e2e.GetEnv("OP_STACK_VERSION", stackVersion)
}

func IngestManagerFeatureContext(s *godog.Suite) {
	imts := IngestManagerTestSuite{
		Fleet: FleetTestSuite{},
	}
	serviceManager := services.NewServiceManager()

	s.Step(`^the "([^"]*)" Kibana setup has been executed$`, imts.Fleet.kibanaSetupHasBeenExecuted)
	s.Step(`^an agent is deployed to Fleet$`, imts.Fleet.anAgentIsDeployedToFleet)
	s.Step(`^the agent is listed in Fleet as online$`, imts.Fleet.theAgentIsListedInFleetAsOnline)
	s.Step(`^system package dashboards are listed in Fleet$`, imts.Fleet.systemPackageDashboardsAreListedInFleet)
	s.Step(`^there is data in the index$`, imts.Fleet.thereIsDataInTheIndex)
	s.Step(`^the "([^"]*)" process is "([^"]*)" on the host$`, imts.Fleet.processStateOnTheHost)
	s.Step(`^the agent is un-enrolled$`, imts.Fleet.theAgentIsUnenrolled)
	s.Step(`^the agent is not listed as online in Fleet$`, imts.Fleet.theAgentIsNotListedAsOnlineInFleet)
	s.Step(`^there is no data in the index$`, imts.Fleet.thereIsNoDataInTheIndex)
	s.Step(`^the agent is re-enrolled on the host$`, imts.Fleet.theAgentIsReenrolledOnTheHost)
	s.Step(`^the enrollment token is revoked$`, imts.Fleet.theEnrollmentTokenIsRevoked)
	s.Step(`^an attempt to enroll a new agent fails$`, imts.Fleet.anAttemptToEnrollANewAgentFails)

	s.BeforeSuite(func() {
		log.Debug("Installing ingest-manager runtime dependencies")

		workDir, _ := os.Getwd()
		profileEnv = map[string]string{
			"stackVersion":     stackVersion,
			"kibanaConfigPath": path.Join(workDir, "configurations", "kibana.config.yml"),
		}

		profile := "ingest-manager"
		err := serviceManager.RunCompose(true, []string{profile}, profileEnv)
		if err != nil {
			log.WithFields(log.Fields{
				"profile": profile,
			}).Error("Could not run the runtime dependencies for the profile.")
		}

		minutesToBeHealthy := 3 * time.Minute
		healthy, err := e2e.WaitForElasticsearch(minutesToBeHealthy)
		if !healthy {
			log.WithFields(log.Fields{
				"error":   err,
				"minutes": minutesToBeHealthy,
			}).Error("The Elasticsearch cluster could not get the healthy status")
		}

		healthyKibana, err := e2e.WaitForKibana(minutesToBeHealthy)
		if !healthyKibana {
			log.WithFields(log.Fields{
				"error":   err,
				"minutes": minutesToBeHealthy,
			}).Error("The Kibana instance could not get the healthy status")
		}
	})
	s.BeforeScenario(func(*messages.Pickle) {
		log.Debug("Before Ingest Manager scenario")

		imts.Fleet.CleanupAgent = false
	})
	s.AfterSuite(func() {
		log.Debug("Destroying ingest-manager runtime dependencies")
		profile := "ingest-manager"

		err := serviceManager.StopCompose(true, []string{profile})
		if err != nil {
			log.WithFields(log.Fields{
				"error":   err,
				"profile": profile,
			}).Warn("Could not destroy the runtime dependencies for the profile.")
		}
	})
	s.AfterScenario(func(*messages.Pickle, error) {
		log.Debug("After Ingest Manager scenario")

		if imts.Fleet.CleanupAgent {
			serviceName := "elastic-agent"

			services := []string{serviceName}

			err := serviceManager.RemoveServicesFromCompose("ingest-manager", services, profileEnv)
			if err != nil {
				log.WithFields(log.Fields{
					"service": serviceName,
				}).Error("Could not stop the service.")
			}

			log.WithFields(log.Fields{
				"service": serviceName,
			}).Debug("Service removed from compose.")
		}
	})
}

// IngestManagerTestSuite represents a test suite, holding references to the pieces needed to run the tests
type IngestManagerTestSuite struct {
	Fleet FleetTestSuite
}
