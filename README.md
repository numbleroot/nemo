# Nemo

Nemo debugs Distributed Systems by analyzing provenance graphs obtained during fault injection.


## Running Nemo

We require two things for running Nemo: a [Go](https://golang.org/doc/install) and a [Docker](https://docs.docker.com/install/overview/) installation. Preferrably, both come from your system's package manager, if available. Make sure to start the Docker daemon afterwards.

Once, these two dependencies are installed and properly configured, run:
```
user@system $   git clone git@github.com:numbleroot/nemo.git ${GOPATH}/src/github.com/numbleroot/nemo
```

Change into the newly created repository directory and execute:
```
user@system $   sudo docker-compose build
user@system $   make build
```
This should take care of preparing the environment and building the Nemo executable.

Finally, with a Molly execution performed prior to this process, run Nemo as follows:
```
user@system $   ./nemo -faultInjOut <PATH TO EXISTING MOLLY EXECUTION>
```

Nemo should debug the Molly execution now. If all goes well, you will be referred to a prepared webpage report to open in your browser.
