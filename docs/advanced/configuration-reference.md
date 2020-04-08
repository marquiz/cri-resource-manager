---
title: "Configuration Reference"
layout: default
---

# Configuration reference

## Configuration flow

How active configuration is retrieved determined

### Node agent
- default config
- group configs
### Cached configuration
### Command line options
### Error handling



## Configuration file

### `policy`

**Active** specifies the active policy.
```yaml
policy:
  Active: static
```

**AvailableResources** specifies the available hardware resources.
**ReservedResources** specifies the hardware resources reserved for system and
kube tasks.
Currently, only CPU resources are supported. CPUs may be specified as a cpuset
or as a numerical value, similar to Kubernetes resource quantities. Not all
policies use these configuration settins. See the policy-specific documentation
for details.

```yaml
policy:
  AvailableResources:
    cpu: cpuset:0-63
  ReservedResources:
    cpu: cpuset:0-3
    # Alternative ways to specify CPUs:
    #cpu: 4
    #cpu: 4000m
```

### `policy.static`

**RelaxedIsolation** controls whether isolated CPUs are preferred for Guarenteed
Pods.

```yaml
policy:
  static:
    RelaxedIsolation: true
```

### `policy.static-plus`

### `policytopology-aware`

### `policy.static-pools`

### `policy.eda`

### `control`

### `control.blockio`

### `control.rdt`

### `blockio`

### `rdt`

### `instrumentation`

### `rdt`

### `blockio`

### `log`

### `dump`
