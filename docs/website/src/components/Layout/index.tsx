/**
 * Copyright (c) Facebook, Inc. and its affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

import React from "react";
import clsx from "clsx";
import ErrorBoundary from "@docusaurus/ErrorBoundary";
import {
  PageMetadata,
  SkipToContentFallbackId,
  ThemeClassNames,
} from "@docusaurus/theme-common";
import { useKeyboardNavigation } from "@docusaurus/theme-common/internal";
import SkipToContent from "@theme/SkipToContent";
import AnnouncementBar from "@theme/AnnouncementBar";
import Footer from "../Footer";
import LayoutProvider from "@theme/Layout/Provider";
import ErrorPageContent from "@theme/ErrorPageContent";
import type { Props } from "@theme/Layout";
import styles from "./styles.module.css";
// import CookieConsent from "react-cookie-consent";

export default function Layout(props: Props): JSX.Element {
  const {
    children,
    noFooter,
    wrapperClassName: wrapperClassName = "page font--jakarta",
    // Not really layout-related, but kept for convenience/retro-compatibility
    title,
    description,
  } = props;

  useKeyboardNavigation();

  return (
    <LayoutProvider>
      {/* <CookieConsent
      //debug={true} 
      location="bottom"
      style={{ background: '#123', textAlign: 'left' }}
      expires={365}
      declineButtonText='I decline'
      hideOnAccept={true}   
      enableDeclineButton
      flipButtons
      buttonText='I accept'     
      >
      This site uses essential and customised cookies. Check our <a href="/privacy-policy">privacy policy</a> for more detailed information.

    </CookieConsent> */}

      <PageMetadata title={title} description={description} />

      <SkipToContent />

      <AnnouncementBar />

      <div
        id={SkipToContentFallbackId}
        className={clsx(
          ThemeClassNames.wrapper.main,
          styles.mainWrapper,
          wrapperClassName
        )}
      >
        <ErrorBoundary fallback={(params) => <ErrorPageContent {...params} />}>
          {children}
        </ErrorBoundary>
      </div>

      {!noFooter && (
        <Footer
          emailItems={[
            {
              email: "office@web7.md",
              emailLink: "office@web7.md",
            },
          ]}
          logoSrc="/img/logo.png"
          socialItems={[
            {
              icon: "flaticon-github",
              url: "https://github.com/kndpio",
            },
          ]}
        />
      )}
    </LayoutProvider>
  );
}
