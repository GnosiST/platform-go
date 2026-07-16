import React from 'react';
import Link from '@docusaurus/Link';

export default function Home(): JSX.Element {
  return (
    <main className="platform-home">
      <section className="platform-hero">
        <div className="container platform-hero__grid">
          <div>
            <p className="platform-kicker">GO OPERATIONS FOUNDATION / 0.1</p>
            <h1>Build the platform layer once.</h1>
            <p className="platform-hero__lede">
              A business-neutral Go foundation for capabilities, identity, authorization, contracts and production operations.
            </p>
            <div className="platform-actions">
              <Link className="button button--primary button--lg" to="/docs/intro">Start with the docs</Link>
              <a className="button button--secondary button--lg" href="https://github.com/GnosiST/platform-go">View on GitHub</a>
            </div>
          </div>
          <div className="platform-architecture" aria-label="Platform request flow diagram">
            <div className="platform-architecture__label">REQUEST FLOW</div>
            <div className="platform-architecture__line"><span>Identity</span><b>+</b><span>Tenant context</span></div>
            <div className="platform-architecture__arrow">↓</div>
            <div className="platform-architecture__line"><span>Capability contract</span><b>→</b><span>Policy boundary</span></div>
            <div className="platform-architecture__arrow">↓</div>
            <div className="platform-architecture__line platform-architecture__line--accent"><span>Service / data plane</span><b>→</b><span>GORM runtime</span></div>
          </div>
        </div>
      </section>
      <section className="container platform-section">
        <div className="platform-section__heading">
          <p className="platform-kicker">WHAT SHIPS IN THE BASE</p>
          <h2>Contracts before customization.</h2>
        </div>
        <div className="platform-pillars">
          <article><span>01</span><h3>Capability manifests</h3><p>Declare resources, routes, permissions, lifecycle and stability in a machine-checkable contract.</p></article>
          <article><span>02</span><h3>Secure defaults</h3><p>JWT sessions, Casbin authorization, tenant boundaries, audit trails and sensitive-data controls.</p></article>
          <article><span>03</span><h3>Production gates</h3><p>OpenAPI, codegen previews, migration checks and release evidence keep the foundation explainable.</p></article>
        </div>
      </section>
      <section className="platform-section platform-section--dark">
        <div className="container platform-section__split">
          <div><p className="platform-kicker">FOR DOWNSTREAM TEAMS</p><h2>Keep business code at the edge.</h2></div>
          <p>Attach your own capability package through the public ports. The platform owns the shared mechanics; your domain owns its data, workflows and UI.</p>
        </div>
      </section>
    </main>
  );
}
