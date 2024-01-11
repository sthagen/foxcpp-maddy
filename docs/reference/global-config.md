# Global configuration directives

These directives can be specified outside of any
configuration blocks and they are applied to all modules.

Some directives can be overridden on per-module basis (e.g. hostname).

### state_dir _path_
Default: `/var/lib/maddy`

The path to the state directory. This directory will be used to store all
persistent data and should be writable.

---

### runtime_dir _path_
Default: `/run/maddy`

The path to the runtime directory. Used for Unix sockets and other temporary
objects. Should be writable.

---

### hostname _domain_ 
Default: not specified

Internet hostname of this mail server. Typicall FQDN is used. It is recommended
to make sure domain specified here resolved to the public IP of the server.

---

### auth_map _module-reference_
Default: `identity`

Use the specified table to translate SASL usernames before passing it to the
authentication provider.

Before username is looked up, it is normalized using function defined by
`auth_map_normalize`.

Note that `auth_map` does not affect the storage account name used. You probably
should also use `storage_map` in IMAP config block to handle this.

This directive is useful if used authentication provider does not support
using emails as usernames but you still want users to have separate mailboxes
on separate domains. In this case, use it with `email_localpart` table:

```
    auth_map email_localpart
```

With this configuration, `user@example.org` and `user@example.com` will use
`user` credentials when authenticating, but will access `user@example.org` and
`user@example.com` mailboxes correspondingly. If you want to also accept
`user` as a username, use `auth_map email_localpart_optional`.

If you want `user@example.org` and `user@example.com` to have the same mailbox,
also set `storage_map` in IMAP config block to use `email_localpart`
(or `email_localpart_optional` if you want to also accept just "user"):

```
    storage_map email_localpart
```

In this case you will need to create storage accounts without domain part in
the name:

```
maddy imap-acct create user # instead of user@example.org
```

---

### auth_map_normalize _function_
Default: `auto`

Normalization function to apply to SASL usernames before mapping
them to storage accounts.

Available options:

- `auto`                    `precis_casefold_email` for valid emails, `precis_casefold` otherwise.
- `precis_casefold_email`   PRECIS UsernameCaseMapped profile + U-labels form for domain
- `precis_casefold`         PRECIS UsernameCaseMapped profile for the entire string
- `precis_email`            PRECIS UsernameCasePreserved profile + U-labels form for domain
- `precis`                  PRECIS UsernameCasePreserved profile for the entire string
- `casefold`                Convert to lower case
- `noop`                    Nothing

---

### autogenerated_msg_domain _domain_
Default: not specified

Domain that is used in From field for auto-generated messages (such as Delivery
Status Notifications).

---

### tls `file` _cert-file_ _pkey-file_ | _module-reference_ | `off`
Default: not specified

Default TLS certificate to use for all endpoints.

Must be present in either all endpoint modules configuration blocks or as
global directive.

You can also specify other configuration options such as cipher suites and TLS
version. See maddy-tls(5) for details. maddy uses reasonable
cipher suites and TLS versions by default so you generally don't have to worry
about it.

---

### tls_client { ... }
Default: not specified

This is optional block that specifies various TLS-related options to use when
making outbound connections. See TLS client configuration for details on
directives that can be used in it. maddy uses reasonable cipher suites and TLS
versions by default so you generally don't have to worry about it.

---

### log _targets..._ | `off`
Default: `stderr`

Write log to one of more "targets".

The target can be one or the following:

- `stderr` –  Write logs to stderr.
- `stderr_ts` – Write logs to stderr with timestamps.
- `syslog` – Send logs to the local syslog daemon.
- _file path_ – Write (append) logs to file.

Example:

```
log syslog /var/log/maddy.log
```

**Note:** Maddy does not perform log files rotation, this is the job of the
logrotate daemon. Send SIGUSR1 to maddy process to make it reopen log files.

---

### debug _boolean_ 
Default: `no`

Enable verbose logging for all modules. You don't need that unless you are
reporting a bug.
