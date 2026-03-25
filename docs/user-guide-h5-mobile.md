# HalaOS Mobile (H5) — Employee User Guide

> **Version:** 1.0 | **Updated:** March 2026
> **URL:** https://halaos.com/m/
> **Supported:** Any mobile browser (Chrome, Safari, Samsung Internet)

---

## Table of Contents

1. [Getting Started](#1-getting-started)
2. [Home](#2-home)
3. [Attendance](#3-attendance)
4. [Leave](#4-leave)
5. [Payslips](#5-payslips)
6. [Profile & Settings](#6-profile--settings)
7. [Notifications](#7-notifications)
8. [AI Assistant](#8-ai-assistant)
9. [Tips & Troubleshooting](#9-tips--troubleshooting)

---

## 1. Getting Started

### 1.1 Accessing the Mobile App

1. Open your mobile browser.
2. Navigate to **https://halaos.com/m/**
3. Bookmark or add to your home screen for quick access.

> **Tip:** On iPhone, tap the Share button → "Add to Home Screen". On Android, tap the three-dot menu → "Add to Home screen".

### 1.2 Login

1. Enter your **email** and **password** provided by your HR administrator.
2. Tap **Sign In**.
3. You will be redirected to the **Home** screen.

### 1.3 Navigation

The app has a **bottom navigation bar** with 5 tabs:

| Tab | Icon | Page |
|-----|------|------|
| **Home** | House | Dashboard overview |
| **Clock** | Clock | Attendance & clock in/out |
| **Leave** | Calendar | Leave balance & requests |
| **Pay** | Bill | Payslips & salary |
| **Me** | Person | Profile & settings |

A floating **AI bubble** (blue chat icon) appears on every page for quick access to the AI assistant.

---

## 2. Home

**Route:** `/m/`

Your personal dashboard with a quick overview of today's status.

### 2.1 What You See

- **Greeting** — Personalized time-based greeting (Good morning/afternoon/evening, {your name})
- **Clock Status Card** — Shows your current attendance status:
  - "Not Clocked In" — You haven't clocked in today
  - "Clocked In at 8:30 AM" — Your clock-in time
  - "Clocked Out at 5:30 PM" — Your clock-out time
- **Leave Balance** — Horizontal scrolling cards showing remaining days for each leave type (Vacation, Sick, etc.)

### 2.2 New Employee Onboarding Checklist

If you're a new employee, you'll see a **Getting Started** checklist:

| Step | Action |
|------|--------|
| 1. Complete Profile | Update your personal information |
| 2. Clock In | Record your first clock-in |
| 3. View Leave | Check your leave balance |
| 4. View Payslip | Access your latest payslip |
| 5. Try AI Chat | Ask the AI assistant a question |

Tap any step to go directly to that feature. The progress bar updates as you complete each step. Tap **Skip** to dismiss.

### 2.3 Quick Actions

A grid of shortcut buttons:

| Action | What It Does |
|--------|-------------|
| **Apply Leave** | Jump to the leave application form |
| **View Payslips** | Go to your payslip history |
| **Notifications** | Open your notification center (with unread badge) |
| **My Info** | View your profile |
| **AI Assistant** | Chat with the AI for HR questions |

### 2.4 AI Quick Ask

Scrollable suggestion tags at the top — tap any to instantly ask the AI:
- "Remind me to clock in"
- "What's my leave balance?"
- Other context-aware suggestions

### 2.5 Pull to Refresh

Pull down on the Home screen to refresh all data.

---

## 3. Attendance

**Route:** `/m/attendance`

Record your work hours and view attendance history.

### 3.1 Clocking In

1. Tap the **Clock** tab in the bottom navigation.
2. You'll see a large **live clock** showing the current time.
3. Tap the big round **Clock In** button (blue).
4. If geofencing is enabled, the app will request your location:
   - "Location acquired" — You're within the office geofence
   - "Getting location..." — Waiting for GPS
   - "Location denied" — You need to enable location permissions
5. Once clocked in, the button changes to **Clock Out** (green).

### 3.2 Clocking Out

1. Tap the **Clock Out** button (green) when you're done for the day.
2. Your clock-out time is recorded.

### 3.3 Today's Summary

Below the clock button, you'll see:
- **Clock In:** 8:30 AM
- **Clock Out:** 5:30 PM (or "—" if not yet clocked out)

### 3.4 Attendance Records

Scroll down to see your attendance history:
- Each record shows the date, clock-in time, clock-out time, and source
- Records are loaded with infinite scroll (pull up to load more)
- If no records exist, you'll see an empty state with a prompt to clock in

### 3.5 AI Quick Ask

Suggested AI questions on this page:
- "Am I late today?"
- "Show my attendance this week"
- "Generate my attendance report"

---

## 4. Leave

**Route:** `/m/leave`

Manage your leave balance, apply for time off, and track requests.

### 4.1 Leave Balance (Tab 1)

View all your leave types and remaining balance:

| Leave Type | Total | Used | Remaining |
|-----------|-------|------|-----------|
| Vacation Leave | 15 | 3 | **12** |
| Sick Leave | 15 | 1 | **14** |
| ... | ... | ... | ... |

### 4.2 Apply Leave (Tab 2)

**How to apply for leave:**

1. Tap the **Apply** tab.
2. Select the **Leave Type** (tap to open the picker).
3. Choose a **Start Date** (tap to open the calendar).
4. Choose an **End Date** (tap to open the calendar).
5. Enter a **Reason** for your leave.
   - Tap **AI Suggest** for AI-generated reason suggestions — select one to auto-fill.
6. Tap **Submit**.

> **AI Prefill:** If you accessed this page from the AI assistant, the form may already be partially filled in. A blue banner indicates AI-prefilled data.

### 4.3 Leave History (Tab 3)

View all your past and current leave requests:

- Each card shows: Leave type, date range, number of days, and status
- **Status badges:**
  - **Pending** — Awaiting manager approval
  - **Approved** — Leave approved
  - **Rejected** — Leave denied
  - **Cancelled** — You cancelled the request

**Cancelling a Request:**
- Swipe left on a **Pending** request to reveal the **Cancel** button
- Only pending requests can be cancelled

---

## 5. Payslips

**Route:** `/m/payslips`

View your salary details and download payslips.

### 5.1 Payslip List

Each card shows:
- **Pay Period** — e.g., "Nov 15 – Dec 15, 2024"
- **Pay Date** — When you were paid
- **Net Pay** — Your take-home amount (highlighted)

Tap any card to open the detail view.

### 5.2 Payslip Detail

A bottom sheet pops up showing the full breakdown:

**Summary:**
| Item | Amount |
|------|--------|
| Basic Salary | ₱25,000.00 |
| Gross Pay | ₱28,500.00 |
| Total Deductions | ₱5,200.00 |
| **Net Pay** | **₱23,300.00** |

**Earnings Breakdown:**
- Basic Salary
- Overtime Pay
- Holiday Pay
- Allowances
- Bonuses

**Deductions Breakdown:**
- Withholding Tax
- SSS Contribution
- PhilHealth
- Pag-IBIG
- Loan Repayment

### 5.3 Download PDF

Tap the **Download PDF** button at the bottom of the detail view to save your payslip as a PDF file.

### 5.4 AI Quick Ask

Suggested AI questions on this page:
- "Show my latest payslip"
- "Why is my pay different this month?"
- "Simulate my salary with overtime"

---

## 6. Profile & Settings

**Route:** `/m/profile`

Manage your personal information and app settings.

### 6.1 Personal Information

Your profile card shows:
- **Avatar** — Circular icon with your initial
- **Full Name**
- **Role** — Your system role (Employee, Manager, etc.)

Below that:
- Email
- Department
- Position
- Employee ID

### 6.2 Telegram Bot Integration

Connect your Telegram account for notifications and self-service commands.

**To connect:**
1. Tap **Connect Telegram**.
2. A 6-character **link code** appears (e.g., `A1B2C3`).
3. Open Telegram and find the **HalaOS bot**.
4. Send `/link A1B2C3` to the bot.
5. Once connected, you'll see "Connected" with your Telegram username.

**To disconnect:**
1. Tap the **Disconnect** button (red) next to your Telegram username.

> The link code expires after 10 minutes. Generate a new one if it expires.

### 6.3 Change Password

1. Tap **Change Password**.
2. Enter your **Current Password**.
3. Enter a **New Password**.
4. **Confirm** the new password.
5. Tap **Submit**.

### 6.4 Language

Tap **Language** to toggle between:
- **English**
- **中文 (Chinese)**

Your preference is saved and applied immediately.

### 6.5 Dark Mode

Toggle the **Dark Mode** switch to change the app theme.

### 6.6 Notification Settings

Tap to configure which notifications you receive.

### 6.7 Logout

Tap **Logout** at the bottom. Confirm when prompted. You'll be redirected to the login page.

---

## 7. Notifications

**Route:** `/m/notifications`

View all your system notifications.

### 7.1 Notification Types

| Type | Color | Examples |
|------|-------|---------|
| **Leave** | Green | "Your leave request was approved" |
| **Payroll** | Orange | "Your payslip for Dec 2024 is ready" |
| **Approval** | Red | "You have a pending approval" |
| **General** | Blue | "Company announcement posted" |

### 7.2 Managing Notifications

- **Swipe Left** on a notification to reveal the **Mark as Read** button
- **Mark All Read** — Tap the button in the top-right corner
- **Unread Indicator** — Unread notifications have a light blue background
- **Pull to Refresh** — Pull down to check for new notifications
- **Infinite Scroll** — Scroll to load older notifications

### 7.3 AI Quick Ask

Suggested AI questions:
- "Summarize my notifications"
- "Any pending approvals?"
- "What needs my attention?"

---

## 8. AI Assistant

**Route:** `/m/ai-chat`

Your AI-powered HR assistant available from any page via the floating chat bubble.

### 8.1 Starting a Conversation

There are three ways to access the AI:

1. **Floating Bubble** — Tap the blue chat icon in the bottom-right corner (available on all pages)
2. **Quick Actions** — Tap "AI Assistant" on the Home screen
3. **AI Quick Ask** — Tap any suggestion tag on any page to start a conversation with that question

### 8.2 Chat Interface

- **Type your question** in the text input at the bottom
- Tap **Send** to submit
- The AI responds in real-time with streaming text
- **Suggestion chips** appear when starting a new chat — tap one to get started

### 8.3 What You Can Ask

**Attendance:**
- "Am I late today?"
- "Show my attendance this week"
- "What time did I clock in yesterday?"

**Leave:**
- "What's my leave balance?"
- "I want to take a vacation next week"
- "Can I take leave on Friday?"

**Payroll:**
- "Show my latest payslip"
- "Why is my pay different this month?"
- "How much overtime did I work?"

**General HR:**
- "What are the company holidays?"
- "How do I change my password?"
- "What are my benefits?"
- "Explain the leave policy"

### 8.4 Agent Picker

Tap the **grid icon** in the top navigation to switch between AI agents:
- **HR Assistant** — General HR questions
- **Payroll Advisor** — Salary and tax questions
- **Leave Advisor** — Leave policies and balance

### 8.5 Chat Sessions

- **Token Balance** — Shown in the top-right (e.g., "125K")
- **New Chat** — Tap the **+** icon to start a fresh conversation
- **Chat History** — Tap the **clock icon** to view past conversations
  - Swipe left on a session to delete it
  - Tap a session to resume it

### 8.6 Feedback

After each AI response, you can tap:
- **Thumbs Up** — Helpful response
- **Thumbs Down** — Not helpful

This helps improve the AI over time.

---

## 9. Tips & Troubleshooting

### 9.1 Tips for Best Experience

- **Add to Home Screen** for an app-like experience (no browser toolbar)
- **Enable Location** for geofenced clock-in to work properly
- **Pull to Refresh** on any page if data seems stale
- **Use AI Quick Ask** tags for common questions instead of typing
- **Check Notifications** regularly for approvals and announcements

### 9.2 Common Issues

| Problem | Solution |
|---------|----------|
| Can't clock in | Check if you're within the office geofence. Enable location permissions in your browser settings. |
| Location error | Go to your phone's Settings → Privacy → Location → Enable for your browser. |
| Page not loading | Pull to refresh, or check your internet connection. |
| Can't cancel leave | Only **Pending** requests can be cancelled. Approved requests require HR intervention. |
| Forgot password | Contact your HR administrator to reset your password. |
| Telegram not connecting | Make sure you're messaging the correct bot and the link code hasn't expired (10 min). |
| AI not responding | Check your token balance. If depleted, contact your administrator. |
| Payslip not showing | Payslips are only available after your employer runs payroll. Check with HR. |

### 9.3 Supported Languages

- **English** (default)
- **中文** (Chinese)

Switch in **Me** → **Language**.

### 9.4 Browser Compatibility

| Browser | Status |
|---------|--------|
| Chrome (Android/iOS) | Fully supported |
| Safari (iOS) | Fully supported |
| Samsung Internet | Fully supported |
| Firefox (Android) | Supported |
| Edge (Android) | Supported |

### 9.5 Data Privacy

- Your location is only used during clock-in/out (not tracked continuously)
- AI conversations are private to your account
- All data is transmitted over HTTPS

---

*For additional help, use the AI Assistant or contact your HR administrator.*
