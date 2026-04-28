import React, {useEffect, useRef, useState} from 'react';
import Layout from '@theme/Layout';
import Link from '@docusaurus/Link';
import Head from '@docusaurus/Head';
import '../css/landing.css';

const INSTALL_CMD = 'curl -sL "https://raw.githubusercontent.com/web-seven/overlock/refs/heads/main/scripts/install.sh" | sh';

function CopyButton({text, mini}) {
  const [copied, setCopied] = useState(false);
  const timer = useRef(null);

  const onClick = async () => {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      const ta = document.createElement('textarea');
      ta.value = text;
      ta.style.position = 'fixed';
      ta.style.opacity = '0';
      document.body.appendChild(ta);
      ta.select();
      try { document.execCommand('copy'); } catch {}
      document.body.removeChild(ta);
    }
    setCopied(true);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => setCopied(false), 1700);
  };

  useEffect(() => () => { if (timer.current) clearTimeout(timer.current); }, []);

  return (
    <button
      className={`copy${mini ? ' copy--mini' : ''}${copied ? ' is-copied' : ''}`}
      onClick={onClick}
      aria-label="Copy install command"
      type="button"
    >
      {!mini && <span className="copy__lbl"></span>}
      <span className="copy__icn copy__icn--c">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <rect x="9" y="9" width="13" height="13" rx="1" ry="1" />
          <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
        </svg>
      </span>
      <span className="copy__icn copy__icn--ok">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <polyline points="20 6 9 17 4 12" />
        </svg>
      </span>
    </button>
  );
}

function StickyInstall() {
  const [visible, setVisible] = useState(false);
  const [dismissed, setDismissed] = useState(false);

  useEffect(() => {
    if (typeof window === 'undefined') return;
    if (sessionStorage.getItem('overlock_stuck3_dismissed') === '1') {
      setDismissed(true);
      return;
    }
    const onScroll = () => {
      setVisible(window.scrollY > window.innerHeight * 0.85);
    };
    window.addEventListener('scroll', onScroll, {passive: true});
    onScroll();
    return () => window.removeEventListener('scroll', onScroll);
  }, []);

  if (dismissed) return null;

  const onDismiss = () => {
    setDismissed(true);
    try { sessionStorage.setItem('overlock_stuck3_dismissed', '1'); } catch {}
  };

  return (
    <div className={`stuck${visible ? ' is-visible' : ''}`} aria-hidden={!visible}>
      <i className="dot dot--ok"></i>
      <span className="stuck__lbl">launch</span>
      <code>{INSTALL_CMD}</code>
      <CopyButton text={INSTALL_CMD} mini />
      <button className="stuck__x" aria-label="Dismiss" onClick={onDismiss} type="button">×</button>
    </div>
  );
}

function Counter({to, dur = 1100}) {
  const ref = useRef(null);
  const [val, setVal] = useState(0);
  const started = useRef(false);

  useEffect(() => {
    if (!ref.current) return;
    if (typeof IntersectionObserver === 'undefined') {
      animate();
      return;
    }
    const io = new IntersectionObserver((entries) => {
      entries.forEach((e) => {
        if (e.isIntersecting && !started.current) {
          started.current = true;
          animate();
          io.disconnect();
        }
      });
    }, {threshold: 0.4});
    io.observe(ref.current);
    return () => io.disconnect();

    function animate() {
      const start = performance.now();
      const tick = (now) => {
        const p = Math.min(1, (now - start) / dur);
        const eased = 1 - Math.pow(1 - p, 3);
        setVal(Math.round(to * eased));
        if (p < 1) requestAnimationFrame(tick);
        else setVal(to);
      };
      requestAnimationFrame(tick);
    }
  }, [to, dur]);

  const padded = String(val).padStart(String(to).length, '0');
  return <span ref={ref} data-count={to}>{padded}</span>;
}

function useReveal() {
  useEffect(() => {
    if (typeof window === 'undefined') return;
    const targets = Array.from(document.querySelectorAll([
      '.landing-root .hud-lbl',
      '.landing-root .hero__title',
      '.landing-root .hero__lede',
      '.landing-root .launch',
      '.landing-root .hero__actions',
      '.landing-root .tile',
      '.landing-root .panel__title',
      '.landing-root .panel__copy',
      '.landing-root .panel__terminal',
      '.landing-root .engines__h',
      '.landing-root .eng',
      '.landing-root .wgmap',
      '.landing-root .reg__card',
      '.landing-root .cmp',
      '.landing-root .tele__cell',
      '.landing-root .final__title',
      '.landing-root .final__actions',
    ].join(',')));
    targets.forEach((t) => t.classList.add('reveal'));
    if (!('IntersectionObserver' in window)) {
      targets.forEach((t) => t.classList.add('is-in'));
      return;
    }
    const io = new IntersectionObserver((entries) => {
      entries.forEach((e) => {
        if (e.isIntersecting) {
          e.target.classList.add('is-in');
          io.unobserve(e.target);
        }
      });
    }, {rootMargin: '0px 0px -8% 0px', threshold: 0.05});
    targets.forEach((t) => io.observe(t));
    return () => io.disconnect();
  }, []);
}

export default function Home() {
  useReveal();
  useEffect(() => {
    if (typeof document === 'undefined') return;
    document.body.classList.add('landing-body');
    return () => document.body.classList.remove('landing-body');
  }, []);
  const year = new Date().getFullYear();

  return (
    <Layout
      description="A CLI mission-control for Crossplane. Provision Kubernetes environments, install providers, attach remote nodes over WireGuard-encrypted SSH, and hot-reload local packages — from one command line."
    >
      <Head>
        <title>Overlock // Mission Control for Crossplane</title>
      </Head>
      <div className="landing-root">
        <div className="space" aria-hidden="true">
          <div className="space__nebula"></div>
          <div className="space__grid"></div>
          <div className="space__stars" id="stars"></div>
          <div className="space__scan"></div>
        </div>

        <main id="top">
          {/* HERO */}
          <section className="hero">
            <div className="wrap hero__wrap">
              <div className="hero__brackets" aria-hidden="true">
                <span className="bk bk--tl"></span>
                <span className="bk bk--tr"></span>
                <span className="bk bk--bl"></span>
                <span className="bk bk--br"></span>
              </div>

              <h1 className="hero__title">
                Mission control<br />
                for <span className="acc">Crossplane.</span>
              </h1>

              <p className="hero__lede">
                A single Go binary that provisions a Crossplane environments, attaches remote nodes over WireGuard-encrypted SSH, and hot-reloads local packages while you build.<br />
                Multi-engine. Multi-node. <span className="mk">Open source &amp; MIT licensed.</span>
              </p>

              <div className="launch" id="install">
                <header className="launch__head">
                  <span className="launch__chip">
                    <i className="dot dot--ok"></i>
                    <b>LAUNCH_SEQUENCE</b>
                    <em>· one line · macOS / linux / windows</em>
                  </span>
                  <span className="launch__meta"><b>v</b>1.0.2&nbsp;·&nbsp;<b>released</b>&nbsp;2026-04-25</span>
                </header>
                <div className="launch__body">
                  <span className="launch__prompt">$</span>
                  <code>{INSTALL_CMD}</code>
                  <CopyButton text={INSTALL_CMD} />
                </div>
              </div>

              <div className="hero__actions">
                <Link className="btn btn--solid btn--lg" to="/docs/guide/getting-started">
                  <span>get started</span><span className="btn__a">↗</span>
                </Link>
                <a className="btn btn--ghost btn--lg" href="https://github.com/web-seven/overlock" target="_blank" rel="noopener noreferrer">
                  <span>star on github</span><span className="btn__a">↗</span>
                </a>
                <a className="btn btn--ghost btn--lg" href="https://discord.gg/W7AsrUb5GC" target="_blank" rel="noopener noreferrer">
                  <span>discord</span><span className="btn__a">↗</span>
                </a>
              </div>
            </div>
          </section>

          {/* DASHBOARD */}
          <section className="dash" id="dash">
            <div className="wrap">
              <div className="seclbl">
                <span className="seclbl__id">[ DASH_00 ]</span>
                <span className="seclbl__t">capability_overview</span>
                <span className="seclbl__r"></span>
                <span className="seclbl__c">07&nbsp;modules</span>
              </div>

              <div className="dash__grid">
                <a className="tile tile--xl" href="#env">
                  <header>
                    <span className="tile__id">01</span>
                    <span className="tile__cat">// quick_env_setup</span>
                    <i className="dot dot--ok tile__dot"></i>
                  </header>
                  <h3>Quick Environment Setup</h3>
                  <p>Single-command provisioning of a fully configured Crossplane environment. Cluster, engine, package manager — bootstrapped automatically.</p>
                  <span className="tile__more">open module&nbsp;→</span>
                </a>

                <a className="tile" href="#env">
                  <header>
                    <span className="tile__id">02</span>
                    <span className="tile__cat">// multi_engine</span>
                    <i className="dot dot--ok tile__dot"></i>
                  </header>
                  <h3>Multi-Engine</h3>
                  <p>KinD, K3s, K3d, K3s-Docker — pick the distribution that fits your workflow.</p>
                </a>

                <a className="tile tile--accent" href="#nodes">
                  <header>
                    <span className="tile__id">03</span>
                    <span className="tile__cat">// remote_nodes</span>
                    <i className="dot dot--ok tile__dot"></i>
                  </header>
                  <h3>Multi-Node &amp; Remote Nodes</h3>
                  <p>Add Linux machines as worker nodes via SSH. Inter-node traffic encrypted by <em>WireGuard</em> out of the box.</p>
                </a>

                <a className="tile" href="#nodes">
                  <header>
                    <span className="tile__id">04</span>
                    <span className="tile__cat">// cpu_limits</span>
                    <i className="dot dot--ok tile__dot"></i>
                  </header>
                  <h3>CPU Limits</h3>
                  <p>Cap CPU per container node so reconciliation loops don't grind your laptop to a halt. Accepts <code>2</code>, <code>0.5</code>, <code>50%</code>.</p>
                </a>

                <a className="tile" href="#packages">
                  <header>
                    <span className="tile__id">05</span>
                    <span className="tile__cat">// packages</span>
                    <i className="dot dot--ok tile__dot"></i>
                  </header>
                  <h3>Package Management</h3>
                  <p>Install, list and remove Crossplane configurations, providers and functions from any OCI registry.</p>
                </a>

                <a className="tile" href="#packages">
                  <header>
                    <span className="tile__id">06</span>
                    <span className="tile__cat">// live_dev</span>
                    <i className="dot dot--ok tile__dot"></i>
                  </header>
                  <h3>Live Development</h3>
                  <p>Hot-reload local packages. Edit code; see changes reflected in the cluster within seconds.</p>
                </a>

                <a className="tile" href="#packages">
                  <header>
                    <span className="tile__id">07</span>
                    <span className="tile__cat">// registries</span>
                    <i className="dot dot--ok tile__dot"></i>
                  </header>
                  <h3>Registry Integration</h3>
                  <p>Local <em>and</em> remote OCI registries. Same UX. Air-gapped pipelines welcome.</p>
                </a>

                <a className="tile tile--wide" href="#plugins">
                  <header>
                    <span className="tile__id">08</span>
                    <span className="tile__cat">// plugins</span>
                    <i className="dot dot--ok tile__dot"></i>
                  </header>
                  <h3>Plugin System</h3>
                  <p>Drop a binary into <code>~/.config/overlock/plugins</code> and it becomes a first-class subcommand. Build your team's CLI without forking ours.</p>
                </a>
              </div>
            </div>
          </section>

          {/* ENVIRONMENT PANEL */}
          <section className="panel" id="env">
            <div className="wrap">
              <div className="seclbl">
                <span className="seclbl__id">[ PNL_01 ]</span>
                <span className="seclbl__t">quick_environment_setup</span>
                <span className="seclbl__r"></span>
                <span className="seclbl__c">priority_high</span>
              </div>

              <div className="panel__grid">
                <div className="panel__copy">
                  <h2 className="panel__title">
                    Create fully configured Crossplane environments with <span className="acc">a single command.</span>
                  </h2>
                  <p>
                    Overlock handles cluster provisioning, Crossplane installation, and initial configuration automatically. No bootstrap scripts. No checklist. Pick a Kubernetes distribution and Overlock builds the rest.
                  </p>
                  <p>
                    Multi-engine support means the same vocabulary across <em>KinD</em>, <em>K3s</em>, <em>K3d</em> and <em>K3s-Docker</em> — choose the engine that fits your machine and your team.
                  </p>
                  <ul className="panel__bullets">
                    <li>Cluster &amp; Crossplane bootstrapped automatically</li>
                    <li>Pin a specific Crossplane version with <code>--engine-version</code></li>
                    <li>Start, stop, upgrade and delete environments at will</li>
                  </ul>
                </div>

                <div className="panel__terminal">
                  <header className="term__bar">
                    <div className="term__lights"><span></span><span></span><span></span></div>
                    <span className="term__title">~/lab — overlock environment</span>
                    <span className="term__rec"><i className="dot dot--ok"></i>READY</span>
                  </header>
                  <pre className="term__body"><code>
                    <span className="c"># Create a new environment with default settings</span>{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> environment create <span className="s">my-dev-env</span>{'\n'}
                    {'\n'}
                    <span className="c"># Create with a specific Crossplane version</span>{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> --engine-version <span className="s">1.18.0</span> environment create <span className="s">my-dev-env</span>{'\n'}
                    {'\n'}
                    <span className="c"># List environments / start / stop / upgrade / delete</span>{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> environment list{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> environment start   <span className="s">my-dev-env</span>{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> environment stop    <span className="s">my-dev-env</span>{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> environment upgrade <span className="s">my-dev-env</span>{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> environment delete  <span className="s">my-dev-env</span>
                  </code></pre>
                </div>
              </div>

              <div className="engines">
                <header className="engines__h">
                  <span className="hud-lbl"><b>SUB_PANEL</b><em>// supported_engines</em></span>
                  <h3>Four engines. <span className="acc">Same vocabulary.</span></h3>
                </header>
                <div className="engines__tbl">
                  <article className="eng">
                    <header><span className="eng__id">01</span><b>KinD</b></header>
                    <p><em>Kubernetes in Docker.</em><br />Quick local testing.</p>
                    <code>--engine kind</code>
                  </article>
                  <article className="eng">
                    <header><span className="eng__id">02</span><b>K3s</b></header>
                    <p><em>Lightweight Kubernetes.</em><br />Low-resource environments.</p>
                    <code>--engine k3s</code>
                  </article>
                  <article className="eng">
                    <header><span className="eng__id">03</span><b>K3d</b></header>
                    <p><em>K3s in Docker.</em><br />Fast multi-cluster setups.</p>
                    <code>--engine k3d</code>
                  </article>
                  <article className="eng eng--accent">
                    <header><span className="eng__id">04</span><b>K3s-Docker</b></header>
                    <p><em>K3s with Docker containers as nodes.</em><br />Distributed and multi-node environments.</p>
                    <code>--engine k3s-docker</code>
                  </article>
                </div>
              </div>
            </div>
          </section>

          {/* NODES PANEL */}
          <section className="panel panel--alt" id="nodes">
            <div className="wrap">
              <div className="seclbl">
                <span className="seclbl__id">[ PNL_02 ]</span>
                <span className="seclbl__t">multi_node · remote_nodes · wireguard</span>
                <span className="seclbl__r"></span>
                <span className="seclbl__c">k3s-docker</span>
              </div>

              <div className="panel__grid">
                <div className="panel__copy">
                  <h2 className="panel__title">
                    Distributed control planes <span className="acc">across any Linux host.</span>
                  </h2>
                  <p>
                    The <code>k3s-docker</code> engine creates an agentless K3s server with two default agent nodes — <em>workloads</em> for user pods and system services, and <em>engine</em> dedicated to Crossplane, providers, functions, Kyverno and CertManager.
                  </p>
                  <p>
                    Remote nodes join the cluster via SSH. Any Linux host with Docker installed can be added as a worker. Inter-node traffic is encrypted by <span className="mk">WireGuard out of the box.</span>
                  </p>
                  <ul className="panel__bullets">
                    <li>Agentless K3s server, two-tier node scoping</li>
                    <li>SSH-attached remote nodes (any Linux + Docker)</li>
                    <li>WireGuard-encrypted inter-node traffic</li>
                    <li>Containers cleaned up automatically on env delete</li>
                  </ul>
                </div>

                <div className="panel__terminal">
                  <header className="term__bar">
                    <div className="term__lights"><span></span><span></span><span></span></div>
                    <span className="term__title">~/lab — overlock env node</span>
                    <span className="term__rec"><i className="dot dot--ok"></i>WG_ON</span>
                  </header>
                  <pre className="term__body"><code>
                    <span className="c"># Create a k3s-docker environment</span>{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> env create <span className="s">my-env</span> <span className="f">--engine</span> k3s-docker{'\n'}
                    {'\n'}
                    <span className="c"># Add a remote machine as an engine node</span>{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> env node create <span className="s">my-remote-node</span> \{'\n'}
                    {'    '}<span className="f">--env</span> my-env \{'\n'}
                    {'    '}<span className="f">--host</span> 192.168.1.100 \{'\n'}
                    {'    '}<span className="f">--scopes</span> engine{'\n'}
                    {'\n'}
                    <span className="c"># Limit each container to 2 CPU cores</span>{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> env create <span className="s">my-env</span> <span className="f">--engine</span> k3s-docker <span className="f">--cpu</span> 2{'\n'}
                    {'\n'}
                    <span className="c"># Fractional and percentage values supported</span>{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> env create <span className="s">my-env</span> <span className="f">--engine</span> k3s-docker <span className="f">--cpu</span> 0.5{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> env create <span className="s">my-env</span> <span className="f">--engine</span> k3s-docker <span className="f">--cpu</span> 50%
                  </code></pre>
                </div>
              </div>

              <div className="wgmap">
                <span className="hud-lbl"><b>FIG_02</b><em>// inter_node_topology</em></span>
                <div className="wgmap__svg" aria-hidden="true">
                  <svg viewBox="0 0 800 280" preserveAspectRatio="xMidYMid meet">
                    <defs>
                      <marker id="arr" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
                        <path d="M0,0 L10,5 L0,10 z" fill="currentColor" />
                      </marker>
                    </defs>
                    <g className="wg-line">
                      <line x1="200" y1="140" x2="380" y2="80" markerEnd="url(#arr)" />
                      <line x1="200" y1="140" x2="380" y2="200" markerEnd="url(#arr)" />
                      <line x1="200" y1="140" x2="600" y2="60" markerEnd="url(#arr)" />
                      <line x1="200" y1="140" x2="600" y2="220" markerEnd="url(#arr)" />
                    </g>
                    <g className="wg-node wg-node--core">
                      <rect x="80" y="100" width="180" height="80" rx="2" />
                      <text x="170" y="132" textAnchor="middle">K3S SERVER</text>
                      <text x="170" y="156" textAnchor="middle" className="wg-sub">agentless · control</text>
                    </g>
                    <g className="wg-node">
                      <rect x="380" y="40" width="160" height="64" rx="2" />
                      <text x="460" y="66" textAnchor="middle">workloads</text>
                      <text x="460" y="86" textAnchor="middle" className="wg-sub">local agent</text>
                    </g>
                    <g className="wg-node">
                      <rect x="380" y="160" width="160" height="64" rx="2" />
                      <text x="460" y="186" textAnchor="middle">engine</text>
                      <text x="460" y="206" textAnchor="middle" className="wg-sub">local agent · crossplane</text>
                    </g>
                    <g className="wg-node wg-node--rem">
                      <rect x="600" y="20" width="180" height="64" rx="2" />
                      <text x="690" y="46" textAnchor="middle">remote::node-01</text>
                      <text x="690" y="66" textAnchor="middle" className="wg-sub">SSH + WireGuard</text>
                    </g>
                    <g className="wg-node wg-node--rem">
                      <rect x="600" y="180" width="180" height="64" rx="2" />
                      <text x="690" y="206" textAnchor="middle">remote::node-02</text>
                      <text x="690" y="226" textAnchor="middle" className="wg-sub">192.168.1.100</text>
                    </g>
                    <text x="400" y="270" textAnchor="middle" className="wg-caption">all inter-node traffic encrypted via WireGuard</text>
                  </svg>
                </div>
              </div>
            </div>
          </section>

          {/* PACKAGES PANEL */}
          <section className="panel" id="packages">
            <div className="wrap">
              <div className="seclbl">
                <span className="seclbl__id">[ PNL_03 ]</span>
                <span className="seclbl__t">packages · live_dev · registries</span>
                <span className="seclbl__r"></span>
                <span className="seclbl__c">crossplane.surface</span>
              </div>

              <div className="panel__grid panel__grid--rev">
                <div className="panel__terminal">
                  <header className="term__bar">
                    <div className="term__lights"><span></span><span></span><span></span></div>
                    <span className="term__title">~/lab — overlock packages</span>
                    <span className="term__rec"><i className="dot dot--ok"></i>SYNC</span>
                  </header>
                  <pre className="term__body"><code>
                    <span className="c"># Install a provider</span>{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> provider install <span className="s">xpkg.upbound.io/crossplane-contrib/provider-gcp:v0.22.0</span>{'\n'}
                    {'\n'}
                    <span className="c"># Apply a configuration</span>{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> configuration apply <span className="s">xpkg.upbound.io/devops-toolkit/dot-application:v3.0.31</span>{'\n'}
                    {'\n'}
                    <span className="c"># Apply a function</span>{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> function apply <span className="s">xpkg.upbound.io/crossplane-contrib/function-patch-and-transform:v0.7.0</span>{'\n'}
                    {'\n'}
                    <span className="c"># List installed packages</span>{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> provider list{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> configuration list{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> function list{'\n'}
                    {'\n'}
                    <span className="c"># Live develop · hot reload from local filesystem</span>{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> configuration serve <span className="s">./my-config-package</span>{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> provider serve <span className="s">./my-provider ./cmd/provider</span>{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> function serve <span className="s">./my-function</span>
                  </code></pre>
                </div>

                <div className="panel__copy">
                  <h2 className="panel__title">
                    The full Crossplane surface, <span className="acc">scripted.</span>
                  </h2>
                  <p>
                    Pull, install, version, and remove configurations, providers and functions from any OCI registry — Upbound, GitHub Container Registry, Harbor, or your own. The verbs you already think in: <code>install</code>, <code>apply</code>, <code>list</code>, <code>describe</code>, <code>delete</code>.
                  </p>
                  <p>
                    Run a registry on your machine for development and CI/CD pipelines, or point at any remote OCI host. Same UX either way. <em>Air-gapped teams welcome.</em>
                  </p>
                  <p>
                    For local development, <code>serve</code> commands watch your filesystem and rebuild &amp; reload the package within seconds — a fast feedback loop Crossplane has been missing.
                  </p>
                </div>
              </div>

              <div className="reg" id="plugins">
                <article className="reg__card">
                  <span className="hud-lbl"><b>SUB_PANEL_A</b><em>// registry</em></span>
                  <h3>Local <em>and</em> remote registries</h3>
                  <pre><code>
                    <span className="c"># Local registry for development</span>{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> registry create <span className="f">--local --default</span>{'\n'}
                    {'\n'}
                    <span className="c"># Connect a remote registry</span>{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> registry create \{'\n'}
                    {'    '}<span className="f">--registry-server</span>=<span className="s">registry.example.com</span> \{'\n'}
                    {'    '}<span className="f">--username</span>=<span className="s">myuser</span> <span className="f">--password</span>=<span className="s">***</span>{'\n'}
                    {'\n'}
                    <span className="c"># List configured registries</span>{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> registry list
                  </code></pre>
                </article>

                <article className="reg__card">
                  <span className="hud-lbl"><b>SUB_PANEL_B</b><em>// plugin_system</em></span>
                  <h3>Drop a binary, <em>ship a subcommand</em></h3>
                  <pre><code>
                    <span className="c"># Use a custom plugin path</span>{'\n'}
                    <span className="p">$</span> <span className="k">overlock</span> <span className="f">--plugin-path</span> /path/to/plugins &lt;cmd&gt;{'\n'}
                    {'\n'}
                    <span className="c"># Default plugin path:</span>{'\n'}
                    ~/.config/overlock/plugins/{'\n'}
                    ├── overlock-deploy{'\n'}
                    ├── overlock-audit{'\n'}
                    └── overlock-secrets
                  </code></pre>
                </article>
              </div>
            </div>
          </section>

          {/* COMPARE */}
          <section className="compare" id="compare">
            <div className="wrap">
              <div className="seclbl">
                <span className="seclbl__id">[ PNL_05 ]</span>
                <span className="seclbl__t">comparable_instruments</span>
                <span className="seclbl__r"></span>
                <span className="seclbl__c">april_2026</span>
              </div>

              <h2 className="sec-title">
                The <span className="acc">platform-builder's</span> control surface.
              </h2>

              <div className="cmp">
                <table>
                  <thead>
                    <tr>
                      <th>capability</th>
                      <th className="is-us">overlock</th>
                      <th>kubectl + helm</th>
                      <th>crossplane CLI</th>
                      <th>up CLI</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr><td>single-command env</td><td className="is-us">✓ <i>yes</i></td><td className="no">manual</td><td className="no">manual</td><td>✓ <i>yes</i></td></tr>
                    <tr><td>multi-engine</td><td className="is-us">✓ <i>kind/k3s/k3d/k3s-docker</i></td><td>any</td><td>any</td><td className="mid">limited</td></tr>
                    <tr><td>multi-node + remote SSH</td><td className="is-us">✓ <i>WireGuard</i></td><td className="no">no</td><td className="no">no</td><td className="no">no</td></tr>
                    <tr><td>CPU per-node limits</td><td className="is-us">✓ <i>--cpu 2 / 0.5 / 50%</i></td><td className="no">manual</td><td className="no">no</td><td className="no">no</td></tr>
                    <tr><td>package management</td><td className="is-us">✓ <i>built-in</i></td><td className="no">manual</td><td className="mid">limited</td><td>✓ <i>built-in</i></td></tr>
                    <tr><td>live-reload dev loop</td><td className="is-us">✓ <i>serve cmds</i></td><td className="no">no</td><td className="no">no</td><td className="mid">partial</td></tr>
                    <tr><td>local + remote registries</td><td className="is-us">✓ <i>both</i></td><td className="no">manual</td><td className="mid">remote only</td><td>✓ <i>both</i></td></tr>
                    <tr><td>plugin system</td><td className="is-us">✓ <i>~/.config/overlock/plugins</i></td><td className="no">—</td><td className="no">no</td><td className="no">no</td></tr>
                    <tr><td>vendor lock-in</td><td className="is-us">✓ <i>none · MIT</i></td><td>none</td><td>none</td><td className="no">SaaS</td></tr>
                  </tbody>
                </table>
              </div>
            </div>
          </section>

          {/* TELEMETRY */}
          <section className="tele">
            <div className="wrap">
              <div className="seclbl">
                <span className="seclbl__id">[ TLM_06 ]</span>
                <span className="seclbl__t">vital_signs</span>
                <span className="seclbl__r"></span>
                <span className="seclbl__c">live</span>
              </div>

              <div className="tele__grid">
                <div className="tele__cell">
                  <span className="tele__lbl">install&nbsp;→&nbsp;running</span>
                  <span className="tele__num"><Counter to={60} /><sub>s</sub></span>
                  <span className="tele__bar"><i style={{'--w': '96%'}}></i></span>
                </div>
                <div className="tele__cell tele__cell--accent">
                  <span className="tele__lbl">engines</span>
                  <span className="tele__num"><Counter to={4} /><sub>·&nbsp;kind/k3s/k3d/k3s-docker</sub></span>
                  <span className="tele__bar"><i style={{'--w': '100%'}}></i></span>
                </div>
                <div className="tele__cell">
                  <span className="tele__lbl">binary</span>
                  <span className="tele__num"><Counter to={1} /><sub>·&nbsp;static · no daemon</sub></span>
                  <span className="tele__bar"><i style={{'--w': '18%'}}></i></span>
                </div>
                <div className="tele__cell">
                  <span className="tele__lbl">plugins</span>
                  <span className="tele__num">∞<sub>·&nbsp;drop a binary</sub></span>
                  <span className="tele__bar"><i style={{'--w': '100%'}}></i></span>
                </div>
              </div>
            </div>
          </section>

          {/* FINAL */}
          <section className="final">
            <div className="wrap final__wrap">
              <div className="hero__brackets" aria-hidden="true">
                <span className="bk bk--tl"></span><span className="bk bk--tr"></span>
                <span className="bk bk--bl"></span><span className="bk bk--br"></span>
              </div>

              <span className="hud-lbl"><i className="dot dot--ok"></i><b>READY_TO_LAUNCH</b><em>// final_sequence</em></span>

              <h2 className="final__title">
                Sixty seconds from now,<br />
                <span className="acc">the platform is yours.</span>
              </h2>

              <div className="launch launch--final">
                <header className="launch__head">
                  <span className="launch__chip"><i className="dot dot--ok"></i><b>EXEC_THIS</b><em>· macOS / linux / windows</em></span>
                  <span className="launch__meta"><b>v</b>1.0.2&nbsp;·&nbsp;<b>2026-04-25</b></span>
                </header>
                <div className="launch__body">
                  <span className="launch__prompt">$</span>
                  <code>{INSTALL_CMD}</code>
                  <CopyButton text={INSTALL_CMD} />
                </div>
                <footer className="launch__foot">
                  <span><i className="dot dot--ok"></i>MIT licensed</span><i>—</i>
                  <span><i className="dot dot--ok"></i>no telemetry</span><i>—</i>
                  <span><i className="dot dot--ok"></i>works offline</span>
                </footer>
              </div>

              <div className="final__actions">
                <a className="btn btn--solid btn--lg" href="https://github.com/web-seven/overlock/releases" target="_blank" rel="noopener noreferrer">
                  <span>releases</span><span className="btn__a">↗</span>
                </a>
                <a className="btn btn--ghost btn--lg" href="https://discord.gg/W7AsrUb5GC" target="_blank" rel="noopener noreferrer">
                  <span>discord</span><span className="btn__a">↗</span>
                </a>
                <a className="btn btn--ghost btn--lg" href="https://github.com/web-seven/overlock" target="_blank" rel="noopener noreferrer">
                  <span>github</span><span className="btn__a">↗</span>
                </a>
              </div>
            </div>
          </section>
        </main>

        <StickyInstall />

        <span style={{display: 'none'}} aria-hidden="true">{year}</span>
      </div>
    </Layout>
  );
}
