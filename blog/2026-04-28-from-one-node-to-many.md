---
slug: from-one-node-to-many
title: "From One Node to Many: Where Crossplane Wants to Live"
authors: [overlock]
tags: [environments, crossplane, distributed, wireguard]
---

> "A distributed system is one in which the failure of a computer you didn't even know existed can render your own computer unusable."
>
> Leslie Lamport

Every Crossplane story starts the same way. You spin up a single-node cluster on your laptop, install Crossplane, and watch your first composition reconcile. It's almost magical, until you try to run real applications next to it.

<!-- truncate -->

## Single node is fine, until you add the rest of the stack

Crossplane itself behaves perfectly well on a single node. The control plane reconciles, the providers reconcile, the compositions do their thing. The problem isn't Crossplane. The problem is everything else you actually want to run next to it.

Local development is the most important phase of the whole project. Long before any of this lands in a cloud, you need to see your app talk to the resources Crossplane is provisioning, watch the wiring with your own eyes, break it on purpose, fix it, break it again. That's where the real work happens. And the laptop has to host it: Crossplane, its providers, every microservice you're building, the databases they hit, the message bus, the auth stub. Memory and CPU run out fast. Crossplane is a polite citizen, but it's still a control plane with reconcilers watching CRDs. It eats space. So do your apps. They eat the same space, and something gives.

The instinct is to scale down: fewer providers, smaller workloads, mock more, run less. That's a tax on the work itself. The thing you came to test gets distorted to fit the machine, and what you end up validating is a smaller, faker version of the system you actually care about.

## Move what doesn't need to be local, off the laptop

There's a quieter answer. Stop treating the laptop as the place where everything lives. Treat it as the workbench. Push everything that doesn't *need* to be in front of you out to other nodes, and keep on the laptop only the thing you're actively touching this afternoon.

That can start small: move Crossplane out. Just to a node that isn't your laptop. A spare box on the network, an old NUC, a teammate's idle workstation. And if you don't happen to have any of those lying around, any SSH-reachable Linux host does the job, including a cheap cloud VM at a few dollars a month. The point isn't to avoid the cloud entirely, it's to avoid having to build a *production* cloud setup just to develop. Let *that* node run the engine: Crossplane, providers, functions, Kyverno, CertManager, all the heavy reconcilers. The laptop suddenly has its CPU back, and the control plane is on stable infrastructure that doesn't have to share a fan with VS Code.

It doesn't stop there. The same gesture works for a workloads node, for a node that hosts your databases, for one that runs the message broker, for whatever else takes resources without ever needing your eyes on it. Each piece lands where it makes sense. The laptop keeps shrinking down toward the one microservice you're rewriting, the dashboard you're inspecting, the test you're stepping through. The supporting cast lives elsewhere, reachable, observable, but no longer fighting your editor for memory.

## The microservice topology, made real before cloud

This is exactly what Kubernetes was made for and exactly what microservice architecture promised. Each role on its own infrastructure, talking over a network, scheduled where it makes sense. Crossplane already lets you wire the pieces together as composable resources. A distributed environment is what makes that wiring real instead of theoretical. The control plane is a real control plane on its own host, the data services run on theirs, the workloads on another, and the bit you're developing sits right under your fingertips. The same shape you'll see in production, except the network hop is to your closet instead of `eu-west-2`.

## Adding a remote node, in one command

Overlock collapses each join into a single sentence. One for the engine:

```sh
overlock env node create my-engine \
  --env my-env \
  --host 192.168.1.100 \
  --scopes engine
```

Another for a workloads host. Another for a database host. Run them as you grow the topology, drop them as you shrink it back down. Behind each line, a remote Linux box goes through the entire ceremony: SSH in, Docker if it's missing, an agent container, a WireGuard tunnel back to the server, CNI wiring, registration. The node shows up in `kubectl get nodes` and the control plane never realises it's two hops away over an encrypted overlay. You shape the cluster around the work, not the other way round.

## What you end up with

The picture you end up with is the picture Crossplane was always meant to project. Control plane on stable infrastructure. Data and infra services on their own hosts. The laptop holding only what you're developing or watching, fast to iterate, easy to throw away. The microservice topology stops being a slide and starts being something you can `kubectl get`, break, and watch heal.

A single binary, a handful of commands, and the local-and-cloud rehearsal becomes a local-and-remote rehearsal *of* the cloud. Same shape, no AWS bill, full speed of iteration. That's what local development looks like when distributed environments are this cheap to summon, and when "what's on the laptop" can finally be just the part of the system you actually want to see.
