import type { Metadata, Viewport } from "next";
import type { ReactNode } from "react";
import { Geist } from "next/font/google";
import { preconnect } from "react-dom";
import Script from "next/script";
import "./globals.css";
import { AuthProvider } from "@/components/auth-provider";
import { ProfileGate } from "@/components/profile-gate";
import { Header } from "@/components/header";
import { BottomNav } from "@/components/bottom-nav";
import { CompetitionPrefetch } from "@/components/competition-prefetch";
import { LeaveGuardProvider } from "@/components/leave-guard";
import { ConfirmProvider } from "@/components/confirm-dialog";
import { AmbientBackground } from "@/components/ambient-background";
import { InstallGuide } from "@/components/install-guide";
import { siteDescription, siteName, siteUrl } from "@/lib/site";
import { JsonLd } from "@/components/json-ld";
import { umamiBeforeSendName, umamiBeforeSendScript } from "@/lib/analytics";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  metadataBase: new URL(siteUrl),
  title: {
    default: `${siteName}｜陸上競技の匿名コミュニティ`,
    template: `%s｜${siteName}`,
  },
  description: siteDescription,
  applicationName: siteName,
  openGraph: {
    type: "website",
    siteName,
    locale: "ja_JP",
    title: `${siteName}｜陸上競技の匿名コミュニティ`,
    description: siteDescription,
  },
  twitter: {
    card: "summary",
  },
  appleWebApp: {
    capable: true,
    title: "TrackTalk",
    // black-translucent だと iOS 26 のホーム画面アプリで、画面の下がステータスバーの高さ分だけ
    // 描かれずに空き、下に固定したバーも浮く(WebKit bug 301108)。ステータスバーは iOS に描かせる。
    statusBarStyle: "default",
  },
  verification: {
    google: "erl1lifh6-0oXq1ecreI-JSTA70vLV_03V2Cwx4b8_k",
  },
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  viewportFit: "cover",
  themeColor: "#16243a",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  preconnect("https://upload.wikimedia.org");
  preconnect("https://thumb.wikimedia.org");

  return (
    <html lang="ja" className={`${geistSans.variable} h-full antialiased`}>
      <body className="isolate flex min-h-full flex-col">
        <JsonLd data={{ "@context": "https://schema.org", "@type": "WebSite", name: siteName, url: siteUrl, description: siteDescription, inLanguage: "ja" }} />
        {/* Umami より先に読み込ませる。送る URL から保護者のリンクのトークンを外す。 */}
        <script dangerouslySetInnerHTML={{ __html: umamiBeforeSendScript }} />
        <Script
          defer
          src="https://cloud.umami.is/script.js"
          data-website-id="0442ad3b-c5a2-45d8-825e-09bb48eded74"
          data-before-send={umamiBeforeSendName}
          strategy="afterInteractive"
        />
        <AuthProvider>
          <LeaveGuardProvider>
          <ConfirmProvider>
          <ProfileGate />
          <AmbientBackground />
          <CompetitionPrefetch />
          <Header />
          <main className="mx-auto w-full max-w-5xl flex-1 px-4 py-6">
            {children}
          </main>
          <BottomNav />
          <InstallGuide />
          </ConfirmProvider>
          </LeaveGuardProvider>
        </AuthProvider>
      </body>
    </html>
  );
}
