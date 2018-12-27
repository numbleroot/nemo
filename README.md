# Nemo

Nemo debugs Distributed Systems by analyzing provenance graphs obtained during fault injection.


## Protocol Case Studies

[Here](https://github.com/numbleroot/nemo/tree/master/case-studies) we provide the Dedalus code for the six case studies we performed as part of the evaluation of our [CIDR 2019 paper](https://people.ucsc.edu/~palvaro/p122-oldenburg-cidr19.pdf).


## Running Nemo

We require two things for running Nemo: a [Go](https://golang.org/doc/install) and a [Docker](https://docs.docker.com/install/overview/) installation. Preferably, both come from your system's package manager, if available. Make sure to start the Docker daemon afterwards.

Once, these two dependencies are installed and properly configured, run:
```
user@system $  git clone git@github.com:numbleroot/nemo.git ${GOPATH}/src/github.com/numbleroot/nemo
```

To build and run auxiliary components, execute the following commands:
```
user@system $  cd ${GOPATH}/src/github.com/numbleroot/nemo
user@system $  sudo docker-compose -f docker-compose.yml build
user@system $  sudo docker-compose -f docker-compose.yml up -d
user@system $  make build
```
This should take care of preparing the environment and building the Nemo executable. Verify via `sudo docker-compose -f docker-compose.yml ps` that the Neo4J container defined in [docker-compose.yml](https://github.com/numbleroot/nemo/blob/master/docker-compose.yml) is running correctly.

Finally, having run Molly (see [below](https://github.com/numbleroot/nemo#integrating-with-molly)) on the target distributed system prior to the following action and Molly successfully identifying a bug, run Nemo as follows:
```
user@system $  ./nemo -faultInjOut <PATH TO EXISTING MOLLY EXECUTION>
```

Nemo should debug the Molly execution now. If all goes well, you will be referred to a prepared webpage report to open in your browser.


### Integrating with Molly

In case you rely on [Molly](https://github.com/palvaro/molly) for finding bugs (as we did in our CIDR paper), we require a slightly modified set of output files and format. Please check out the following fork: [Molly fork](https://github.com/KamalaRamas/molly/tree/graphing) (Kamala's fork of Molly set to latest commit on branch `graphing`).
