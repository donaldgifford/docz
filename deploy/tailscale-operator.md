# GitHub webhooks through the Tailscale operator

The `docz` Helm chart has no Tailscale in it. How an install is exposed is
up to its operator (DESIGN-0018 §5). This page covers one way that works:
GitHub delivers webhooks to docz-api through a
[Tailscale Kubernetes operator](https://tailscale.com/kb/1236/kubernetes-operator)
Ingress with Funnel enabled. It replaces the sidecar that `charts/docz-api`
carried up to 0.9.0.

This page covers only the docz side: which chart values to set and what the
tailnet must allow. How the operator itself behaves (its proxies, tags, and
Funnel) is documented by Tailscale in its
[Ingress guide](https://tailscale.com/kb/1439/kubernetes-operator-cluster-ingress)
and [Funnel docs](https://tailscale.com/kb/1223/funnel).

## Chart values

Only the API needs the public edge. GitHub posts to `/webhooks/github` on
docz-api, and nothing public ever needs the site. Give the API an Ingress
with the operator's class:

```yaml
api:
  ingress:
    enabled: true
    className: tailscale
    annotations:
      # Expose on the public internet through Funnel, not only the tailnet.
      tailscale.com/funnel: "true"
    hosts:
      - host: docz-api
        paths:
          - path: /webhooks/github
            pathType: Prefix
    tls:
      # The operator takes the node's MagicDNS name from here.
      - hosts: [docz-api]
```

The Ingress sends traffic only to `<release>-docz-api`, so the site is never
published through Funnel. Limiting the path to `/webhooks/github` keeps
`/api/v1` off the internet as well. For internal access to the API or the
site, use their own `httpRoute` or `ingress` blocks.

## The webhook URL

Set the GitHub App's webhook URL to:

```text
https://docz-api.<tailnet>.ts.net/webhooks/github
```

Here `docz-api` is the `tls.hosts` entry above, and `<tailnet>` is your
tailnet's DNS name (shown in the admin console under DNS).

## Tailnet policy

If either of these is missing, every delivery fails with a **TLS EOF**:
GitHub reports a connection that closed during the handshake, and nothing
reaches docz-api's logs.

1. **Funnel granted to the proxy's tag.** Add a `nodeAttrs` entry that gives
   the `funnel` attribute to the tag the operator's proxies run as
   (`tag:k8s` unless you configured another):

   ```json
   "nodeAttrs": [
     { "target": ["tag:k8s"], "attr": ["funnel"] }
   ]
   ```

2. **HTTPS certificates enabled** for the tailnet (admin console → DNS →
   HTTPS Certificates). Funnel serves TLS with a certificate for the
   MagicDNS name, and without HTTPS enabled it has none to serve.

To diagnose: `tailscale funnel status` on the proxy shows nothing when the
`nodeAttrs` grant is missing. An Ingress with no `status.loadBalancer`
hostname usually means the operator could not create the proxy. Check the
operator's logs.

## Not yet covered

One Tailscale Ingress (or HTTPRoute) in front of both workloads, with the
site at `/` and the API at `/webhooks`, is a follow-up (DESIGN-0018 Rollout
step 7). Until then the chart has one edge per workload.
