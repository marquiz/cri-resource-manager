---
title: "Introduction"
weight: 1
draft: true
---

# Introduction

The main purpose of this CRI relay/proxy is to apply various (hardware) resource
allocation policies to containers in a system. The relay sits between the kubelet
and the container runtime, relaying request and responses back and forth between
these two, potentially altering requests as they fly by.

The details of how requests are altered depends on which policy is active inside
the relay. There are several policies available, each geared towards a different
set of goals and implementing different hardware allocation strategies.

## CRI proxy

## Node agent

## What CRI-RM is about

## What CRI-RM is NOT about

