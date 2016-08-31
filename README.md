puller
===

Puller is a ridiculously simple, distributed Docker container deployment and CI tool. 

Given you feed it some service definitions (through yaml files, http calls or cli client), it will:

- Pull the image from the repository, and keep it fresh (with gcloud support)
- Deploy the service to the node if the nodename matches the regexp in the service definition
- Watch for service definition change (exposing an additional port? changing image tag? no problem, detected!) and recreate the container if needed
- If you update the service definition on one node, the change will propagate across all nodes
- Easily change to/roll back to a certain tag of the image

First steps
=====

Here is a (made up) example service definition file

``` 
name: cassa
bash: docker run -p=66:55 -restart=always cassandra /bin/sh
repo: cassandra
tag: latest
node: box-*
```

Start the puller daemon with the following command:

```
puller -d -node="box-1" -dir=/dirpath
```

Voila! Puller will make sure your service will be deployed on your local machine and it will keep the imgage fresh. Change to tag name or any other property in this file and Puller compare, download and redeploy or remove if needed.

Getting a feel for a multi node setup
=====

You can play around with a multinode setup locally by launching more instances and using different dirs for the service definitions:

```
puller -d -join=127.0.0.1:7946 -port=7710 -dir=/dirpath2 -node="xob-2"
```

```
puller -d -join=127.0.0.1:7946 -port=7720 -dir=/dirpath3 -node="xob-3"
```

Change the service definitions in any of those dirs, watch it being propagated and the service getting deployed on the appropriate nodes!

A real setup example
=====

This is literally how we use Puller, the "leader" is only used for discovery, once the cluster is established it does not matter if it goes down (ie. no single point of failure)

```
# Bootstrap leader
puller -d  -dir=/var/puller

# Bootstrap siblings
/usr/bin/puller -d -dir=/var/puller -join=consul-seed
```

Pros
=====

- It is ridiculously simple. I don't plan big features. I just want this to work and do its job. All the time.

Cons
=====

- It is single threaded currently - it first pulls, then removes/launches etc. This is not ideal in all scenarios but at least the threads don't have to be rate limited - used to run into exceeding quota with Google Cloud.
