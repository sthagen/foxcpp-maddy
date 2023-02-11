# MAIL FROM and From authorization

Module check.authorize_sender verifies that envelope and header sender addresses belong
to the authenticated user. Address ownership is established via table
that maps each user account to a email address it is allowed to use.
There are some special cases, see user\_to\_email description below.

```
check.authorize_sender {
    prepare_email identity
    user_to_email identity
    check_header yes

    unauth_action reject
    no_match_action reject
    malformed_action reject
    err_action reject

    auth_normalize precis_casefold_email
    from_normalize precis_casefold_email
}
```
```
check {
    authorize_sender { ... }
}
```

## Configuration directives

**Syntax:** user\_to\_email _table_ <br>
**Default:** identity

Table that maps authorization username to the list of sender emails
the user is allowed to use.

In additional to email addresses, the table can contain domain names or
special string "\*" as a value. If the value is a domain - user
will be allowed to use any mailbox within it as a sender address.
If it is "\*" - user will be allowed to use any address.

By default, table.identity is used, meaning that username should
be equal to the sender email.

Before username is looked up via the table, normalization algorithm
defined by auth_normalize is applied to it.

**Syntax:** prepare\_email _table_ <br>
**Default:** identity

Table that is used to translate email addresses before they
are matched against user_to_email values.

Typically used to allow users to use their aliases as sender
addresses - prepare_email in this case should translate
aliases to "canonical" addresses. This is how it is
done in default configuration.

If table does not contain any mapping for the used sender
address, it will be used as is.

**Syntax:** check\_header _boolean_ <br>
**Default:** yes

Whether to verify header sender in addition to envelope.

Either Sender or From field value should match the
authorization identity.

**Syntax:** unauth\_action _action_ <br>
**Default:** reject

What to do if the user is not authenticated at all.

**Syntax:** no\_match\_action _action_ <br>
**Default:** reject

What to do if user is not allowed to use the sender address specified.

**Syntax:** malformed\_action _action_ <br>
**Default:** reject

What to do if From or Sender header fields contain malformed values.

**Syntax:** err\_action _action_ <br>
**Default:** reject

What to do if error happens during prepare\_email or user\_to\_email lookup.

**Syntax:** auth\_normalize _action_ <br>
**Default:** precis\_casefold\_email

Normalization function to apply to authorization username before
further processing.

Available options:
- precis\_casefold\_email   PRECIS UsernameCaseMapped profile + Unicode form for domain
- precis\_casefold         PRECIS UsernameCaseMapped profile for the entire string
- precis\_email            PRECIS UsernameCasePreserved profile + Unicode form for domain
- precis                  PRECIS UsernameCasePreserved profile for the entire string
- casefold                Convert to lower case
- noop                    Nothing

PRECIS profiles are defined by RFC 8265. In short, they make sure
that Unicode strings that look the same will be compared as if they were
the same. CaseMapped profiles also convert strings to lower case.

**Syntax:** from\_normalize _action_ <br>
**Default:** precis\_casefold\_email

Normalization function to apply to email addresses before
further processing.

Available options are same as for auth\_normalize.
