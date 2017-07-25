package main

import (
	"context"
	"flag"

	"fmt"

	"time"

	"errors"

	"os"

	"github.com/digitalocean/godo"
	"github.com/koding/sshkey"
	"golang.org/x/oauth2"
)

type TokenSource struct {
	AccessToken string
}

type stringFlag struct {
	set   bool
	value string
}

func (sf *stringFlag) Set(x string) error {
	sf.value = x
	sf.set = true
	return nil
}

func (sf *stringFlag) String() string {
	return sf.value
}

func (t *TokenSource) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: t.AccessToken,
	}
	return token, nil
}

func createDroplett(ctx context.Context, client *godo.Client, dropletName string, tag string, sshKeyID int) (ip string, err error) {
	createRequest := &godo.DropletCreateRequest{
		Name:   dropletName,
		Region: "nyc3",
		Size:   "512mb",
		Image: godo.DropletCreateImage{
			Slug: "ubuntu-16-04-x64",
		},
		PrivateNetworking: true,
		Tags:              []string{tag},
		SSHKeys: []godo.DropletCreateSSHKey{
			{
				ID: sshKeyID,
			},
		},
	}
	newDroplet, _, err := client.Droplets.Create(ctx, createRequest)
	if err != nil {
		panic(err)
	}
	breakLoop := false
	var dropletNetwork *godo.Networks
	for {
		dropletStatus, _, err := client.Droplets.Get(ctx, newDroplet.ID)
		dropletNetwork = dropletStatus.Networks
		if err != nil {
			panic(err)
		}
		fmt.Printf("Droplet status is %s\n", dropletStatus.Status)
		for _, currentTag := range dropletStatus.Tags {
			if currentTag == tag {
				breakLoop = true
			}
		}
		if breakLoop == true {
			break
		}
		fmt.Println("Droplet tag is not set yet, will check again in 10 seconds")
		time.Sleep(10 * time.Second)
	}
	fmt.Printf("Droplet %s created\n", dropletName)
	for _, network := range dropletNetwork.V4 {
		if network.Type == "public" {
			return network.IPAddress, nil
		}
	}
	return "", errors.New("No Private IP Address")
}

func createFirewalls(ctx context.Context, client *godo.Client) error {
	opt := &godo.ListOptions{
		Page:    1,
		PerPage: 200,
	}
	allDroplets, _, err := client.Droplets.List(ctx, opt)
	if err != nil {
		fmt.Println("failed getting droplets")
		panic(err)
	}
	var privateAddresses []string
	for _, system := range allDroplets {
		for _, network := range system.Networks.V4 {
			if network.Type == "private" {
				privateAddresses = append(privateAddresses, fmt.Sprintf("%s/32", network.IPAddress))
			}
		}
	}

	backendfirewallRequest := &godo.FirewallRequest{
		Name: "BackendFirewall",
		InboundRules: []godo.InboundRule{
			//Allow any communication from private addresses
			{
				Protocol:  "tcp",
				PortRange: "all",
				Sources: &godo.Sources{
					Addresses: privateAddresses,
				},
			},
			{
				Protocol:  "udp",
				PortRange: "all",
				Sources: &godo.Sources{
					Addresses: privateAddresses,
				},
			},
			{
				Protocol: "icmp",
				Sources: &godo.Sources{
					Addresses: privateAddresses,
				},
			},
		},
		OutboundRules: []godo.OutboundRule{
			//Allow all out to private hosts
			{
				Protocol:  "tcp",
				PortRange: "all",
				Destinations: &godo.Destinations{
					Addresses: privateAddresses,
				},
			},
			{
				Protocol:  "udp",
				PortRange: "all",
				Destinations: &godo.Destinations{
					Addresses: privateAddresses,
				},
			},
			//Allow DNS Outbound
			{
				Protocol:  "udp",
				PortRange: "53",
				Destinations: &godo.Destinations{
					Addresses: []string{"0.0.0.0/0", "::/0"},
				},
			},
			//Allow HTTP Outbound
			{
				Protocol:  "tcp",
				PortRange: "80",
				Destinations: &godo.Destinations{
					Addresses: []string{"0.0.0.0/0", "::/0"},
				},
			},
			//Allow HTTPS Outbound
			{
				Protocol:  "tcp",
				PortRange: "443",
				Destinations: &godo.Destinations{
					Addresses: []string{"0.0.0.0/0", "::/0"},
				},
			},
			//Allow Ping Outbound
			{
				Protocol: "icmp",
				Destinations: &godo.Destinations{
					Addresses: []string{"0.0.0.0/0", "::/0"},
				},
			},
		},
		Tags: []string{"Backend_Systems"},
	}
	_, _, err = client.Firewalls.Create(ctx, backendfirewallRequest)
	if err != nil {
		return err
	}
	fmt.Printf("Backend Firewall created sucesfully\n")

	bastionFirewallRequest := &godo.FirewallRequest{
		Name: "BastionFirewall",
		InboundRules: []godo.InboundRule{
			//Allow in from private hosts
			{
				Protocol:  "tcp",
				PortRange: "all",
				Sources: &godo.Sources{
					Addresses: privateAddresses,
				},
			},
			{
				Protocol:  "udp",
				PortRange: "all",
				Sources: &godo.Sources{
					Addresses: privateAddresses,
				},
			},
			{
				Protocol: "icmp",
				Sources: &godo.Sources{
					Addresses: privateAddresses,
				},
			},
			//Allow SSH in
			{
				Protocol:  "tcp",
				PortRange: "22",
				Sources: &godo.Sources{
					Addresses: []string{"0.0.0.0/0", "::/0"},
				},
			},
			//Allow Ping in
			{
				Protocol: "icmp",
				Sources: &godo.Sources{
					Addresses: []string{"0.0.0.0/0", "::/0"},
				},
			},
		},
		OutboundRules: []godo.OutboundRule{
			//Allow all out to private hosts
			{
				Protocol:  "tcp",
				PortRange: "all",
				Destinations: &godo.Destinations{
					Addresses: privateAddresses,
				},
			},
			{
				Protocol:  "udp",
				PortRange: "all",
				Destinations: &godo.Destinations{
					Addresses: privateAddresses,
				},
			},
			//Allow HTTP out
			{
				Protocol:  "tcp",
				PortRange: "80",
				Destinations: &godo.Destinations{
					Addresses: []string{"0.0.0.0/0", "::/0"},
				},
			},
			//Allow HTTPS out
			{
				Protocol:  "tcp",
				PortRange: "443",
				Destinations: &godo.Destinations{
					Addresses: []string{"0.0.0.0/0", "::/0"},
				},
			},
			//Allow SSH out
			{
				Protocol:  "tcp",
				PortRange: "22",
				Destinations: &godo.Destinations{
					Addresses: []string{"0.0.0.0/0", "::/0"},
				},
			},
			//Allow DNS out
			{
				Protocol:  "udp",
				PortRange: "53",
				Destinations: &godo.Destinations{
					Addresses: []string{"0.0.0.0/0", "::/0"},
				},
			},
			//Allow Ping out
			{
				Protocol: "icmp",
				Destinations: &godo.Destinations{
					Addresses: []string{"0.0.0.0/0", "::/0"},
				},
			},
		},
		Tags: []string{"Bastion_Systems"},
	}
	_, _, err = client.Firewalls.Create(ctx, bastionFirewallRequest)
	if err != nil {
		return err
	}
	fmt.Printf("Bastion Firewall created sucesfully\n")
	return nil
}
func main() {
	var pat stringFlag
	flag.Var(&pat, "pat", "Specify your private access token to authenticate with DigitalOcean")
	flag.Parse()

	if !pat.set {
		fmt.Println("-pat is required to be set")
		os.Exit(0)
	}

	// Future use could be allowing the number of front/backend servers to be configurable
	numBackend := 2
	numBastion := 1

	tokenSource := &TokenSource{
		AccessToken: pat.String(),
	}
	oauthClient := oauth2.NewClient(context.Background(), tokenSource)
	client := godo.NewClient(oauthClient)
	ctx := context.TODO()

	//Create SSH Keys
	pubSSHKey, privSSHKey, err := sshkey.Generate()
	fmt.Printf("SSH Keys have been generated\n")
	SSHKeyRequest := &godo.KeyCreateRequest{
		Name:      "MySSHKEy",
		PublicKey: pubSSHKey,
	}
	sshKeyInfo, _, err := client.Keys.Create(ctx, SSHKeyRequest)
	if err != nil {
		panic(err)
	}

	//Create Tags
	tags := []string{"Bastion_Private", "Backend_Systems", "Bastion_Systems"}
	for _, tag := range tags {
		tagCreateRequest := &godo.TagCreateRequest{
			Name: tag,
		}
		fmt.Printf("Creating tag: %s\n", tag)
		_, _, err := client.Tags.Create(ctx, tagCreateRequest)
		if err != nil {
			panic(err)
		}
	}

	//Setup Backend servers
	for i := 1; i <= numBackend; i++ {
		fmt.Printf("Creating backend droplet %d\n", i)
		dropletName := fmt.Sprintf("backendDroplet-%d", i)
		//Don't really care about the backend IPs, but need to do something with the variable
		_, err = createDroplett(ctx, client, dropletName, "Backend_Systems", sshKeyInfo.ID)
		if err != nil {
			panic(err)
		}
	}

	//Setup Bastion server
	var firstBastionIP string
	for i := 1; i <= numBastion; i++ {
		fmt.Printf("Creating Bastion droplet %d\n", i)
		dropletName := fmt.Sprintf("bastionDroplet-%d", i)
		firstBastionIP, err = createDroplett(ctx, client, dropletName, "Bastion_Systems", sshKeyInfo.ID)
		if err != nil {
			if err.Error() == "No Private IP Address" {
				fmt.Printf("Unable to get private IP address for bastion server. Check DO console")
			} else {
				panic(err)
			}
		}
	}

	//setup firewall
	createFirewalls(ctx, client)
	fmt.Printf("Public Key is %s\n", pubSSHKey)
	fmt.Printf("The private key to access the servers is as follows:\n%s\n", privSSHKey)
	fmt.Printf("The IP address of the Bastion server is: %s", firstBastionIP)
}
