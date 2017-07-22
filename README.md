# Create_DoVPG
Create a simple VPC setup with the following configuration.

1 Bastion droplet to be used for access into the VPC
2 Backend droplets
  These droplets can only recieve traffic on the private network but can reach out to the internet via http/https (dns and ping is also allowed).
 
SSH is locked down via sshkeys and a new key is generated each time the environment is stood up. The key and the public IP address of the bastion droplet will be outputed once the environment has been fully provisioned.

**Note For additional security firewall rules should be setup against the private network to allow only ports in use for the services they are providing. By default all private servers can talk to each other over any port in the private network.

## Usage
go get ./...
go build main.go
./main -pat <Digital Ocean Personal Access Token>
