package deployment

import (
	"encoding/json"
	"github.com/openshift/geard/cmd"
	"github.com/openshift/geard/port"
	"io/ioutil"
	"regexp"
	"strings"
	"testing"
)

var localhost = cmd.HostLocator{"127.0.0.1", 0}
var noHosts PlacementStrategy = SimplePlacement(cmd.Locators{})
var oneHost PlacementStrategy = SimplePlacement(cmd.Locators{&localhost})

func loadDeployment(path string) *Deployment {
	body, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return createDeployment(string(body))
}

func createDeployment(body string) *Deployment {
	deployment := &Deployment{}
	decoder := json.NewDecoder(strings.NewReader(body))
	if err := decoder.Decode(deployment); err != nil {
		panic(err)
	}
	return deployment
}

func assignPorts(dep *Deployment) {
	p := 10000
	for i := range dep.Instances {
		instance := &dep.Instances[i]
		for j := range instance.Ports {
			mapping := &instance.Ports[j]
			if mapping.External.Default() {
				mapping.External = port.Port(p)
				p++
			}
		}
	}
}

func TestPrepareDeployment(t *testing.T) {
	dep := createDeployment(`{
    "containers":[
      {
        "name":"web",
        "count":2,
        "image":"pmorie/sti-html-app",
        "publicports":[
          {"internal":8080,"external":0}
        ]
      },
      {
        "name":"db",
        "count":3,
        "image":"pmorie/sti-db-app"
      }
    ]
  }`)
	if _, _, err := dep.Describe(noHosts); err != nil {
		t.Fatal("Should not return error when describing with no hosts")
	}
	next, removed, err := dep.Describe(oneHost)
	if err != nil {
		t.Fatal("Error when describing one host", err)
	}
	if len(next.Instances) != 5 {
		t.Fatalf("Expected %d instances, got %d", 5, len(next.Instances))
	}
	for i := range next.Instances {
		if next.Instances[i].On == nil {
			t.Fatalf("Instance %d has an empty host %+v", i+1, next.Instances[i])
		}
	}
	if len(removed) != 0 {
		t.Fatal("Should not have removed instances", removed)
	}
}

func TestPrepareDeploymentExternal(t *testing.T) {
	dep := createDeployment(`{
    "containers":[
      {
        "name":"web",
        "count":2,
        "image":"pmorie/sti-html-app",
        "publicports":[
          {"internal":8080,"external":80}
        ]
      }
    ]
  }`)
	next, removed, err := dep.Describe(oneHost)
	if err != nil {
		t.Fatal("Error when describing one host", err)
	}
	if len(next.Instances) != 2 {
		t.Fatalf("Expected %d instances, got %d", 5, len(next.Instances))
	}
	if len(next.Instances[0].Ports) != 1 || next.Instances[0].Ports[0].External != 80 {
		t.Fatalf("External port not preserved across instantiation: %+v", next.Instances)
	}
	if len(removed) != 0 {
		t.Fatal("Should not have removed instances", removed)
	}
}

func TestPrepareDeploymentRemoveMissing(t *testing.T) {
	dep := createDeployment(`{
    "containers":[
      {
        "name":"web",
        "count":2,
        "image":"pmorie/sti-html-app"
      }
    ],
    "instances":[
      {
        "id":"foo",
        "from":"db"
      }
    ]
  }`)
	next, removed, err := dep.Describe(oneHost)
	if err != nil {
		t.Fatal("Error when describing one host", err)
	}
	if len(next.Instances) != 2 {
		t.Fatalf("Expected %d instances, got %d", 5, len(next.Instances))
	}
	if len(removed) != 0 {
		t.Fatal("Instances without hosts should be ignored", removed)
	}

	dep.Instances[0].On = &localhost
	next, removed, err = dep.Describe(oneHost)
	if err != nil {
		t.Fatal("Error when describing one host", err)
	}
	if len(next.Instances) != 2 {
		t.Fatalf("Expected %d instances, got %d", 5, len(next.Instances))
	}
	if len(removed) != 1 || removed[0].From != "db" {
		t.Fatalf("Should have removed db instance %+v", removed)
	}
}

func TestPrepareDeploymentError(t *testing.T) {
	dep := createDeployment(`{
    "containers":[
      {
        "name":"web",
        "count":2,
        "image":"pmorie/sti-html-app",
        "publicports":[
          {"internal":8080,"external":0}
        ],
        "links":[
          {"to":"web"}
        ]
      },
      {
        "name":"db",
        "count":3,
        "image":"pmorie/sti-db-app"
      }
    ]
  }`)
	if _, _, err := dep.Describe(oneHost); err != nil {
		t.Fatal("Should not have received an error", err.Error())
	}

	dep.Containers[0].Links[0].Ports = []port.Port{port.Port(8081)}
	if _, _, err := dep.Describe(oneHost); err == nil {
		t.Fatal("Should have received an error")
	} else {
		if !regexp.MustCompile("target port 8081 on web is not found").MatchString(err.Error()) {
			t.Fatal("Unexpected error message", err.Error())
		}
	}

	link := &dep.Containers[0].Links[0]
	link.Ports = []port.Port{}
	link.To = "db"
	if _, _, err := dep.Describe(oneHost); err == nil {
		t.Fatal("Should have received an error")
	} else {
		if !regexp.MustCompile("target db has no public ports to link to from web").MatchString(err.Error()) {
			t.Fatal("Unexpected error message", err.Error())
		}
	}

	dep.Containers[1].PublicPorts = port.PortPairs{port.PortPair{port.Port(27017), 0}}
	next, removed, err := dep.Describe(oneHost)
	if err != nil {
		t.Fatal("Should not have received an error", err.Error())
	}
	if len(next.Instances) != 5 {
		t.Fatalf("Expected %d instances, got %d", 5, len(next.Instances))
	}
	if len(next.Instances[0].links) != 3 {
		t.Fatalf("Should have exactly 1 link %+v", next.Instances[0].links)
	}
	if len(removed) != 0 {
		t.Fatal("Should not have removed instances", removed)
	}

	dep.RandomizeIds = true
	dep.Containers[1].PublicPorts = port.PortPairs{port.PortPair{port.Port(27017), 0}}
	dep.Containers[0].Links = append(dep.Containers[0].Links, Link{
		To: "web",
	})
	next, removed, err = dep.Describe(oneHost)
	if err != nil {
		t.Fatal("Should not have received an error", err.Error())
	}
	if len(next.Instances) != 5 {
		t.Fatalf("Expected %d instances, got %d", 5, len(next.Instances))
	}
	if len(next.Instances[0].links) != 5 {
		t.Fatalf("Should have exactly 5 links (2 web links, 3 mongo links) %+v", next.Instances[0].links)
	}
	if len(removed) != 0 {
		t.Fatal("Should not have removed instances", removed)
	}
	if next.Instances[0].Id == "web-1" {
		t.Fatal("Should randomize ids", next.Instances[0])
	}

	// b, _ := json.MarshalIndent(next, "", "  ")
	// t.Log(string(b))
}

func TestPrepareDeploymentInterlink(t *testing.T) {
	dep := loadDeployment("./fixtures/complex_deploy.json")
	changes, _, err := dep.Describe(oneHost)
	if err != nil {
		t.Fatal("Should not have received an error", err)
	}
	if len(changes.Instances) != 5 {
		t.Fatalf("Expected %d instances, got %d", 5, len(changes.Instances))
	}

	assignPorts(changes)
	changes.UpdateLinks()

	for i := range changes.Instances {
		instance := changes.Instances[i]
		for j := range instance.links {
			link := instance.links[j]
			if link.ToPort.Default() {
				t.Fatalf("Expected all link ports to be assigned %s: %+v", instance.Id, link)
			}
		}
	}

	// b, _ := json.MarshalIndent(changes, "", "  ")
	// t.Log(string(b))
}

func TestPrepareDeploymentMongo(t *testing.T) {
	dep := loadDeployment("./fixtures/mongo_deploy.json")
	changes, _, err := dep.Describe(oneHost)
	if err != nil {
		t.Fatal("Should not have received an error", err)
	}
	if len(changes.Instances) != 3 {
		t.Fatalf("Expected %d instances, got %d", 3, len(changes.Instances))
	}

	assignPorts(changes)
	changes.UpdateLinks()

	for i := range changes.Instances {
		instance := changes.Instances[i]
		port := instance.links[0].FromPort
		host := instance.links[0].FromHost
		if "192.168.1.1" != host {
			t.Fatal("Expected first host to be 192.168.1.1", host)
		}
		for j, link := range instance.links {
			if link.ToPort.Default() {
				t.Fatalf("Expected all link ports to be assigned %s: %+v", instance.Id, link)
			}
			if link.FromPort != port {
				t.Fatalf("Expected all link FromPorts to be the same %d: %d", port, link.FromPort)
			}
			if j > 0 && link.FromHost == host {
				t.Fatalf("Expected all link FromHosts to be different %s: %s", host, link.FromHost)
			}
		}
	}
}

func TestReloadDeploymentMongo(t *testing.T) {
	dep := loadDeployment("./fixtures/mongo_deploy_existing.json")
	changes, _, err := dep.Describe(oneHost)
	if err != nil {
		t.Fatal("Should not have received an error", err)
	}
	if len(changes.Instances) != 3 {
		t.Fatalf("Expected %d instances, got %d", 3, len(changes.Instances))
	}

	assignPorts(changes)
	changes.UpdateLinks()

	for i := range changes.Instances {
		instance := changes.Instances[i]
		port := instance.links[0].FromPort
		host := instance.links[0].FromHost
		if "192.168.1.1" != host {
			t.Fatal("Expected first host to be 192.168.1.1", host)
		}
		for j, link := range instance.links {
			if link.ToPort.Default() {
				t.Fatalf("Expected all link ports to be assigned %s: %+v", instance.Id, link)
			}
			if link.FromPort != port {
				t.Fatalf("Expected all link FromPorts to be the same %d: %d", port, link.FromPort)
			}
			if j > 0 && link.FromHost == host {
				t.Fatalf("Expected all link FromHosts to be different %s: %s", host, link.FromHost)
			}
		}
	}

	changes, removed, err := dep.Describe(noHosts)
	if err != nil {
		t.Fatal("Should not have received an error", err)
	}
	if len(changes.Instances) != 0 {
		t.Fatalf("Expected %d instances, got %d", 0, len(changes.Instances))
	}
	if len(removed) != 3 {
		t.Fatalf("Expected to remove %d instances, got %d", 3, len(removed))
	}

	// b, _ := json.MarshalIndent(changes, "", "  ")
	// t.Log(string(b))
}
