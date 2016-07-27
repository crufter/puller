puller
===

Puller is an easy to use, distributed docker container deployment and CI tool. 

Given you feed it some service definitions (through yaml files, http calls or cli client), it will:

- Pull the image from the repository, and keep it fresh
- Deploy the service to the node if the nodename matches the regexp in the service definition
- Watch for service configuration change and recreate the container if needed
- If you update the service configuration on one node, the change will propagate across all nodes
- Easily change to/roll back to a certain tag of the image

First steps
=====

Here is an (pretty useless and made up) example service definition file

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

Voila! Puller will make sure your service will be deployed on your local machine!

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
