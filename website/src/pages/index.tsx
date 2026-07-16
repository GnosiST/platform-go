import React from 'react';
import Link from '@docusaurus/Link';

export default function Home(): JSX.Element {
  return (
    <main className="container margin-vert--xl">
      <h1>platform-go</h1>
      <p>A business-neutral Go operations platform foundation.</p>
      <Link className="button button--primary" to="/docs/intro">
        Read the documentation
      </Link>
    </main>
  );
}
