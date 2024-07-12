import React from 'react';
import CodeBlock from '@theme/CodeBlock';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';

export default function RenderCodeblock() {
  const {
    siteConfig: { customFields },
  } = useDocusaurusContext();
  return (
    <CodeBlock language="bash">
      {`VERSION="${customFields.version}"\nARCH="linux-amd64"\ncurl -Lo ./kndp.tar.gz "https://github.com/kndpio/cli/releases/download/$VERSION/kndp-$VERSION-$ARCH.tar.gz"; tar -xf ./kndp.tar.gz; rm ./kndp.tar.gz\nsudo mv ./kndp /usr/local/bin/kndp`}
    </CodeBlock>
  );
}
