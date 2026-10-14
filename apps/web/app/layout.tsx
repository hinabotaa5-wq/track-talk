import type { Metadata, Viewport } from "next";
import type { ReactNode } from "react";
import { Geist } from "next/font/google";
import { preconnect } from "react-dom";
import Script from "next/script";
import "./globals.css";
import { AuthProvider } from "@/components/auth-provider";
import { Header } from "@/components/header";
import { BottomNav } from "@/components/bottom-nav";
import { CompetitionPrefetch } from "@/components/competition-prefetch";
import { ExpandedPlaybackGuard } from "@/components/expanded-playback-guard";
import { AmbientBackground } from "@/components/ambient-background";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "TrackTalk",
  description: "陸上競技を、もっと深く楽しむコミュニティ",
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  viewportFit: "cover",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  preconnect("https://upload.wikimedia.org");
  preconnect("https://thumb.wikimedia.org");

  return (
    <html lang="ja" className={`${geistSans.variable} h-full antialiased`}>
      <body className="isolate flex min-h-full flex-col">
        <Script
          defer
          src="https://cloud.umami.is/script.js"
          data-website-id="0442ad3b-c5a2-45d8-825e-09bb48eded74"
          strategy="afterInteractive"
        />
        <AuthProvider>
          <AmbientBackground />
          <CompetitionPrefetch />
          <ExpandedPlaybackGuard />
          <Header />
          <main className="mx-auto w-full max-w-5xl flex-1 px-4 py-6">
            {children}
          </main>
          <BottomNav />
        </AuthProvider>
      </body>
    </html>
  );
}
