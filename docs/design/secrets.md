<!-- -*- fill-column: 100 -*- -->
# Secrets handling

**Status: figuring out**

## Motivation and requirements

 * Most of the time, configuration (the sources for Modules) will be protected by authentication and
   authorisation, in the form of an _access key_ secret (whether it's an SSH key, or a password, or
   a token)

 * I want to limit the exposure in the case of a cluster break-in; so ideally, each cluster gets its
   own credential, which can be revoked
   - but you can't or shouldn't e.g., install thousands of deploy keys in GitHub, so the naive
     solution of creating an access key per sync is not ideal

 * Being able to rotate keys, and revoke access, will make the system safer.

The following design sketches outline elements of a solution. They are not necessarily mutually
exclusive.

## Design sketch 1: the simplest thing that works**

The user is responsible for creating and installing an access key (e.g., a deploy key in GitHub),
and putting the private key in a secret in the management cluster alongside their Module
objects. Each Module can refer to a Secret, which is in turn referenced in the ProxyAssemblage, and
copied into the downstream cluster alongside the Assemblage object. The Assemblage controller then
references the secret when it constructs a GitRepository.

This design can be supplemented later with more automation, e.g., to create and install deploy keys.

Advantages:

 - it's simple and it will work fine for toy systems and demos
 - it's easy to see how to build further layers on this, e.g., automation for creating deploy keys

Disadvantages:

 - without more automation, the user has to do a lot of arranging things just so -- they must
   install a deploy key, create a secret, and refer to it correctly.
 - the access key is an important secret, and ends up in lots of clusters in plain text (though may
   be encrypted at rest)
 - the same key is used for every cluster, so revoking access means uninstalling the key, which
   affects every cluster

## Design sketch 2: session keys

The control plane cluster is given the capability, via OAuth, of issuing session authentication
tokens. It can create these without (much) communication to the origin (GitHub). A session token for
each Module is created for each cluster, and renewed on a schedule. The token is copied to the
downstream cluster by the ProxyAssemblage controller.

Advantages:

 - renewal is relatively inexpensive, so you can do it hourly or something
 - an expired token will not break the system or hold syncing up for long
 - rotation happens naturally since session keys must be renewed

Disadvantage:

 - I'm not certain this is possible with GitHub, or other providers (and it probably won't be for
   roll-your-own hosting)
 - Revocation may be complicated

## Design sketch 3: intermediate caching

Configurations are processed and cached, and authentication is with the cache rather than the
origin. This means you can have whatever auth^n scheme works for you, since you are in control of
the infrastructure.

## Other concerns and alternatives

### Naive threat model

Here are some possible attacks:

**An attacker manages to read the access key secret in the downstream**

For example, the attacker gains direct access to a downstream cluster via a stray kubeconfig, and is
able to read a secret created for use by Flux primitives.

This is not necessarily difficult, since it's the path of least resistance to give tenants read
access to secrets in their namespace, and for their syncs to end up in their namespace.

Consequences:

 - the attacker can read the git repository for as long as the access key is valid

Mitigations:

 - use a platform service to encrypt the access key, so you need to be running with a particular IAM
   role to be able to get the plaintext
 - put tenant syncs and secrets in a namespace to which the tenant doesn't by default have permission

### Encrypting access keys and using platform key services

AWS, Azure and GCP (and probably others) have platform services for managing encryption keys. It's
possible to encrypt secrets in the management cluster, then decrypt in the downstream cluster using
a key stored in AWS KMS (for instance).

Is there any benefit to encrypting the secrets? They would be encrypted in transit -- but that's
done over TLS anyway -- and at rest -- but that can be done using KMS encryption provider, at least
in AWS.

One benefit of encrypting access keys is that the secret value is particular to a cluster. If it's
never in plaintext, and the decryption key is available only through the platform, the access key is
harder to obtain even if a cluster is compromised. The access key can be regenerated and
re-encrypted for uncompromised clusters, though this may be time-consuming.

Another benefit is that the platform service can manage rotation and revocation.

## Resources

 * What OAuth does and doesn't do: https://oauth.net/articles/authentication/
 * How GitHub supports OAuth: https://docs.github.com/en/developers/apps/building-oauth-apps/authorizing-oauth-apps
