package dkim

import (
	"crypto"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/emersion/go-message/textproto"
	"github.com/emersion/go-msgauth/dkim"
	"github.com/foxcpp/maddy/buffer"
	"github.com/foxcpp/maddy/config"
	"github.com/foxcpp/maddy/log"
	"github.com/foxcpp/maddy/module"
	"github.com/foxcpp/maddy/target"
)

const Day = 86400 * time.Second

var (
	oversignDefault = []string{
		// Directly visible to the user.
		"Subject",
		"Sender",
		"To",
		"Cc",
		"From",
		"Date",

		// Affects body processing.
		"MIME-Version",
		"Content-Type",
		"Content-Transfer-Encoding",

		// Affects user interaction.
		"Reply-To",
		"In-Reply-To",
		"Message-Id",
		"References",

		// Provide additional security benefit for OpenPGP.
		"Autocrypt",
		"Openpgp",
	}
	signDefault = []string{
		// Mailing list information. Not oversigned to prevent signature
		// breakage by aliasing MLMs.
		"List-Id",
		"List-Help",
		"List-Unsubscribe",
		"List-Post",
		"List-Owner",
		"List-Archive",

		// Not oversigned since it can be prepended by intermediate relays.
		"Resent-To",
		"Resent-Sender",
		"Resent-Message-Id",
		"Resent-Date",
		"Resent-From",
		"Resent-Cc",
	}

	hashFuncs = map[string]crypto.Hash{
		"sha256": crypto.SHA256,
	}
)

type Modifier struct {
	instName string

	domain         string
	selector       string
	signer         crypto.Signer
	oversignHeader []string
	signHeader     []string
	headerCanon    dkim.Canonicalization
	bodyCanon      dkim.Canonicalization
	sigExpiry      time.Duration
	hash           crypto.Hash

	log log.Logger
}

func New(_, instName string, _, inlineArgs []string) (module.Module, error) {
	m := &Modifier{
		instName: instName,
		log:      log.Logger{Name: "sign_dkim"},
	}

	switch len(inlineArgs) {
	case 2:
		m.domain = inlineArgs[0]
		m.selector = inlineArgs[1]
	case 0:
		// whatever
	case 1:
		fallthrough
	default:
		return nil, errors.New("sign_dkim: wrong amount of inline arguments")
	}

	return m, nil
}

func (m *Modifier) Name() string {
	return "sign_dkim"
}

func (m *Modifier) InstanceName() string {
	return m.instName
}

func (m *Modifier) Init(cfg *config.Map) error {
	var (
		hashName        string
		keyPathTemplate string
		newKeyAlgo      string
	)

	cfg.Bool("debug", true, false, &m.log.Debug)
	cfg.String("domain", false, false, m.domain, &m.domain)
	cfg.String("selector", false, false, m.selector, &m.selector)
	cfg.String("key_path", false, false, "dkim_keys/{domain}_{selector}.key", &keyPathTemplate)
	cfg.StringList("oversign_fields", false, false, oversignDefault, &m.oversignHeader)
	cfg.StringList("sign_fields", false, false, signDefault, &m.signHeader)
	cfg.Enum("header_canon", false, false,
		[]string{string(dkim.CanonicalizationRelaxed), string(dkim.CanonicalizationSimple)},
		dkim.CanonicalizationRelaxed, (*string)(&m.headerCanon))
	cfg.Enum("body_canon", false, false,
		[]string{string(dkim.CanonicalizationRelaxed), string(dkim.CanonicalizationSimple)},
		dkim.CanonicalizationRelaxed, (*string)(&m.bodyCanon))
	cfg.Duration("sig_expiry", false, false, 5*Day, &m.sigExpiry)
	cfg.Enum("hash", false, false,
		[]string{"sha256"}, "sha256", &hashName)
	cfg.Enum("newkey_algo", false, false,
		[]string{"rsa4096", "rsa2048", "ed25519"}, "rsa2048", &newKeyAlgo)

	if _, err := cfg.Process(); err != nil {
		return err
	}

	if m.domain == "" {
		return errors.New("sign_domain: domain is not specified")
	}
	if m.selector == "" {
		return errors.New("sign_domain: selector is not specified")
	}

	m.hash = hashFuncs[hashName]
	if m.hash == 0 {
		panic("sign_dkim.Init: Hash function allowed by config matcher but not present in hashFuncs")
	}

	keyValues := strings.NewReplacer("{domain}", m.domain, "{selector}", m.selector)
	keyPath := keyValues.Replace(keyPathTemplate)

	signer, err := m.loadOrGenerateKey(m.domain, m.selector, keyPath, newKeyAlgo)
	if err != nil {
		return err
	}
	m.signer = signer

	return nil
}

func (m *Modifier) fieldsToSign(h textproto.Header) []string {
	// Filter out duplicated fields from configs so they
	// will not cause panic() in go-msgauth internals.
	seen := make(map[string]struct{})

	res := make([]string, 0, len(m.oversignHeader)+len(m.signHeader))
	for _, key := range m.oversignHeader {
		if _, ok := seen[strings.ToLower(key)]; ok {
			continue
		}
		seen[strings.ToLower(key)] = struct{}{}

		// Add to signing list once per each key use.
		for field := h.FieldsByKey(key); field.Next(); {
			res = append(res, key)
		}
		// And once more to "oversign" it.
		res = append(res, key)
	}
	for _, key := range m.signHeader {
		if _, ok := seen[strings.ToLower(key)]; ok {
			continue
		}
		seen[strings.ToLower(key)] = struct{}{}

		// Add to signing list once per each key use.
		for field := h.FieldsByKey(key); field.Next(); {
			res = append(res, key)
		}
	}
	return res
}

type state struct {
	m    *Modifier
	meta *module.MsgMetadata
	log  log.Logger
}

func (m *Modifier) ModStateForMsg(msgMeta *module.MsgMetadata) (module.ModifierState, error) {
	return state{
		m:    m,
		meta: msgMeta,
		log:  target.DeliveryLogger(m.log, msgMeta),
	}, nil
}

func (s state) RewriteSender(mailFrom string) (string, error) {
	return mailFrom, nil
}

func (s state) RewriteRcpt(rcptTo string) (string, error) {
	return rcptTo, nil
}

func (s state) RewriteBody(h textproto.Header, body buffer.Buffer) error {
	id := s.meta.OriginalFrom
	if !strings.Contains(id, "@") {
		id += "@" + s.m.domain
	}

	opts := dkim.SignOptions{
		Domain:                 s.m.domain,
		Selector:               s.m.selector,
		Identifier:             id,
		Signer:                 s.m.signer,
		Hash:                   s.m.hash,
		HeaderCanonicalization: s.m.headerCanon,
		BodyCanonicalization:   s.m.bodyCanon,
		HeaderKeys:             s.m.fieldsToSign(h),
	}
	if s.m.sigExpiry != 0 {
		opts.Expiration = time.Now().Add(s.m.sigExpiry)
	}
	signer, err := dkim.NewSigner(&opts)
	if err != nil {
		s.m.log.Printf("%v", strings.TrimPrefix(err.Error(), "dkim: "))
		return err
	}
	if err := textproto.WriteHeader(signer, h); err != nil {
		s.m.log.Printf("I/O error: %v", err)
		signer.Close()
		return err
	}
	r, err := body.Open()
	if err != nil {
		s.m.log.Printf("I/O error: %v", err)
		signer.Close()
		return err
	}
	if _, err := io.Copy(signer, r); err != nil {
		s.m.log.Printf("I/O error: %v", err)
		signer.Close()
		return err
	}

	if err := signer.Close(); err != nil {
		s.m.log.Printf("%v", strings.TrimPrefix(err.Error(), "dkim: "))
		return err
	}

	h.Add("DKIM-Signature", signer.SignatureValue())

	s.m.log.Debugf("signed, identifier = %s", id)

	return nil
}

func (s state) Close() error {
	return nil
}

func init() {
	module.Register("sign_dkim", New)
}