# HalaPeople employee app (iOS / Android)

One app for every company. Employees install it once, sign in with their
work email, and land in their own company (if the email has accounts at
several companies, sign-in asks which one). The same code also runs as an
installable web app at `/m/`.

## What it does

Clock in/out (suite edition), leave balances and requests, payslips (PDF),
expenses, notifications, profile with two-factor sign-in, and the AI agent
when the server has an AI provider. Every in-app notification is also sent
to the phone when the employee turns on **Me → Phone notifications**.

## Servers

- **HalaPeople cloud**: set `VITE_API_URL=https://<your-api-host>/api` when
  building; employees never see a server field.
- **Self-hosted**: build without `VITE_API_URL`. On first launch the app asks
  for the company's HalaPeople address (e.g. `people.acme.ph`); it can be
  changed from the sign-in screen.

The API already allows the app's origins (`capacitor://localhost`,
`https://localhost`); set `NATIVE_APP_ORIGINS=off` to disable that.

## Build

```bash
cd frontend-mobile
npm install
npm run cap:sync          # builds dist-native and copies it into android/ and ios/
npx cap open android      # Android Studio → Build → Generate Signed Bundle (AAB)
npx cap open ios          # Xcode (macOS) → Product → Archive
```

App ID: `com.halapeople.app` (change `appId` in `capacitor.config.ts` before
the first store upload if you publish under your own name).

## Phone notifications (native app)

The native app receives pushes through Firebase Cloud Messaging, which also
delivers to iPhones through APNs.

1. Create a Firebase project; add an Android app (`com.halapeople.app`) and
   an iOS app with the same bundle ID.
2. Android: put `google-services.json` in `frontend-mobile/android/app/`.
3. iOS: put `GoogleService-Info.plist` in `frontend-mobile/ios/App/App/`,
   enable *Push Notifications* and *Background Modes → Remote notifications*
   in Xcode, and upload an APNs key in Firebase → Project settings → Cloud
   Messaging.
4. Server: Firebase → Project settings → Service accounts → *Generate new
   private key*, then set `FCM_SERVICE_ACCOUNT_FILE=/path/to/key.json` (or
   `FCM_SERVICE_ACCOUNT_JSON`). Without it, the web app's pushes still work
   (Web Push, VAPID keys generated automatically); native devices register
   but receive nothing.

## Store listing checklist

- Apple Developer Program and Google Play Console accounts.
- Icons are in `frontend-mobile/public/` (`icon-512.png`, maskable variant);
  generate store sizes with `npx @capacitor/assets generate`.
- Privacy policy URL (the landing site has `/privacy`), support URL.
- Apple review: provide a demo company and employee login.
- Data safety / privacy labels: name, email, employee ID numbers, payroll
  and leave data, device push token; no tracking, no ads.
