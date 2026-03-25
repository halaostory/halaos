# HalaOS Admin Dashboard — User Guide

> **Version:** 1.0 | **Updated:** March 2026
> **URL:** https://halaos.com
> **Supported browsers:** Chrome, Edge, Safari, Firefox (latest versions)

---

## Table of Contents

1. [Getting Started](#1-getting-started)
2. [Dashboard](#2-dashboard)
3. [People Management](#3-people-management)
4. [Time & Attendance](#4-time--attendance)
5. [Leave Management](#5-leave-management)
6. [Payroll & Compensation](#6-payroll--compensation)
7. [Approvals & Workflows](#7-approvals--workflows)
8. [Talent Development & Performance](#8-talent-development--performance)
9. [Employee Engagement & Culture](#9-employee-engagement--culture)
10. [Compliance & Legal](#10-compliance--legal)
11. [AI & Analytics](#11-ai--analytics)
12. [Virtual Office](#12-virtual-office)
13. [System Administration](#13-system-administration)
14. [Integrations](#14-integrations)
15. [Account & Billing](#15-account--billing)

---

## 1. Getting Started

### 1.1 Login

1. Open **https://halaos.com/login** in your browser.
2. Enter your **email** and **password**.
3. Click **Sign In**.

> First-time super admins will be redirected to the **Setup Wizard** to configure company information.

### 1.2 Setup Wizard

The Setup Wizard walks you through initial system configuration:

| Step | What to Do |
|------|-----------|
| Company Information | Enter company name, TIN, address |
| Departments & Positions | Create your org structure |
| Import Employees | Add your team via CSV or one by one |
| Configure Leave Policies | Set leave types, balances, approval rules |
| Set Up Work Schedules | Define shifts, work hours, rest days |
| Configure Payroll | Set pay periods, salary structure, taxes |
| Run First Payroll | Process your first payroll cycle |

Progress is displayed on the **Getting Started** checklist on the Dashboard. You can dismiss it once all steps are complete.

### 1.3 Roles & Permissions

| Role | Access Level |
|------|-------------|
| **Super Admin** | Full system access — settings, billing, all features |
| **Admin** | Full HR management — employees, payroll, compliance |
| **Manager** | Team management — approvals, reports, department view |
| **Employee** | Self-service — own attendance, leave, payslips, profile |

---

## 2. Dashboard

**Route:** `/`

The Dashboard is your home screen. It provides a real-time overview of your organization.

### 2.1 Key Metrics

- **Total Headcount** — Active employees across all departments
- **Today's Attendance** — Present / absent / late breakdown
- **Pending Approvals** — Leave and overtime requests awaiting action
- **Upcoming Payroll** — Next payroll run date and status

### 2.2 Charts & Visualizations

- **Headcount Trend** — Monthly headcount over the past 12 months
- **Turnover Analysis** — Hire vs. separation rates
- **Department Distribution** — Employee count per department (pie chart)
- **Payroll Overview** — Monthly payroll costs trend

### 2.3 Quick Info Panels

- **Birthdays & Anniversaries** — Upcoming celebrations this month
- **Expiring Documents** — Documents nearing expiration (e.g., visas, permits)
- **Announcements** — Latest company announcements

### 2.4 Getting Started Checklist

Displays for new accounts. Shows completion progress (e.g., "3 / 7 completed"). Each step links to the relevant feature page. Click **Dismiss** to hide after completion.

---

## 3. People Management

### 3.1 Employees

**Route:** `/employees`
**Roles:** Super Admin, Admin, Manager

The central employee database.

**Viewing Employees:**
- Searchable, sortable table with filters for department, position, status
- Click any employee row to view their full profile

**Adding an Employee:**
1. Click **Add Employee** button.
2. Fill in required fields: First Name, Last Name, Email, Department, Position.
3. Optionally add: Phone, Date of Birth, Date of Hire, Salary, Bank Details.
4. Click **Save**.

**Batch Import:**
1. Click **Import CSV**.
2. Download the template file.
3. Fill in employee data and upload the CSV.
4. Review the import preview and confirm.

**Employee Profile** (`/employees/:id`):
- **Personal Information** — Name, contact, emergency contacts
- **Employment Details** — Department, position, hire date, employment type
- **Compensation** — Salary, allowances, bank details
- **Documents** — Uploaded files (contracts, IDs, certificates)
- **Attendance History** — Clock records
- **Leave History** — Past and current requests
- **Performance Reviews** — Ratings and feedback

**Editing an Employee** (`/employees/:id/edit`):
1. Navigate to the employee profile.
2. Click **Edit**.
3. Update any field and click **Save**.

### 3.2 Directory

**Route:** `/directory`
**Roles:** All

An org chart and contact lookup tool.

- Browse by department or search by name
- View contact cards with email, phone, position
- Visual org hierarchy

### 3.3 201 File

**Route:** `/201file`
**Roles:** Super Admin, Admin, Manager

Philippine-standard employee documentation management.

- **Document Types** — Pre-employment requirements, contracts, government IDs, certifications
- **Expiry Tracking** — Automatic alerts for documents nearing expiration
- **Required Documents Checklist** — Track which documents are missing per employee
- **Upload & Download** — Drag-and-drop file upload, PDF preview

### 3.4 Onboarding

**Route:** `/onboarding`
**Roles:** Super Admin, Admin, Manager

Manage new hire onboarding and employee offboarding.

- **Onboarding Checklists** — Create task lists for new hires (IT setup, orientation, document submission)
- **Offboarding Process** — Clearance workflows for departing employees
- **Task Assignment** — Assign checklist items to specific team members
- **Progress Tracking** — Monitor completion rates

---

## 4. Time & Attendance

### 4.1 Attendance

**Route:** `/attendance`
**Roles:** All

- **Clock In / Clock Out** — Manual or automatic (with geofencing)
- **View Today's Status** — See who's present, late, or absent
- **Manual Entry** — Admin can add manual clock entries for corrections

### 4.2 Attendance Records

**Route:** `/attendance/records`
**Roles:** All

- Filter by date range, employee, department
- View clock-in and clock-out times
- Export to Excel

### 4.3 Attendance Report

**Route:** `/attendance/report`
**Roles:** Super Admin, Admin, Manager

- Summary reports by department and date range
- Tardiness and absence trends
- Exportable reports

### 4.4 Daily Time Record (DTR)

**Route:** `/dtr`
**Roles:** Super Admin, Admin, Manager

- Formal DTR reports per employee per period
- Export to PDF or Excel
- Signature verification support

### 4.5 Schedules

**Route:** `/schedules`
**Roles:** Super Admin, Admin, Manager

- **Create Schedule Templates** — Define shift patterns (e.g., 9-6, rotating)
- **Assign Schedules** — Assign templates to employees or departments
- **Manage Rest Days** — Configure weekly rest days
- **Shift Management** — Handle shift swaps and overrides

### 4.6 Geofencing

**Route:** `/geofences`
**Roles:** Super Admin, Admin

- **Define Geofence Zones** — Set office/site coordinates and radius
- **Auto-Clock Rules** — Automatically validate clock-ins within the geofence
- **Multiple Locations** — Support for multiple office sites

---

## 5. Leave Management

### 5.1 Leaves

**Route:** `/leaves`
**Roles:** All

**Filing a Leave Request:**
1. Click **Request Leave**.
2. Select **Leave Type** (Vacation, Sick, etc.).
3. Choose **Start Date** and **End Date**.
4. Enter a **Reason** (AI suggestions available).
5. Click **Submit**.

**Tracking Requests:**
- View all your requests with status: Pending, Approved, Rejected, Cancelled
- Cancel pending requests before approval

### 5.2 Leave Calendar

**Route:** `/leave-calendar`
**Roles:** All

- Visual calendar showing team members on leave
- Conflict detection (overlapping leaves in same department)
- Filter by department or leave type

### 5.3 Leave Encashment

**Route:** `/leave-encashment`
**Roles:** All

- Convert unused leave balance to cash
- Calculate conversion amount
- Submit for approval

### 5.4 Overtime

**Route:** `/overtime`
**Roles:** All

**Filing an Overtime Request:**
1. Click **File OT Request**.
2. Select the **date** and enter **hours**.
3. Provide a **reason**.
4. Click **Submit**.

- Approval workflow managed via the Approvals page
- Payment calculation based on overtime rules (regular, holiday, rest day)

---

## 6. Payroll & Compensation

### 6.1 Payroll

**Route:** `/payroll`
**Roles:** Super Admin, Admin

**Running Payroll:**
1. Click **Create Payroll Run**.
2. Select the **pay period** (e.g., 1st-15th or 16th-30th).
3. Review auto-calculated salaries, deductions, and contributions.
4. Apply any **bonuses** or **adjustments**.
5. Review the **anomaly detection** report (AI flags unusual amounts).
6. Click **Finalize** to lock the payroll run.
7. Generate **payslips** for distribution.

**Features:**
- Automatic tax computation (withholding tax, SSS, PhilHealth, Pag-IBIG)
- 13th month pay calculation
- Overtime and holiday pay computation
- Deduction management (loans, advances, absences)

### 6.2 Payslips

**Route:** `/payslips`
**Roles:** All

- View payslips for each pay period
- Breakdown: Basic Salary, Earnings (allowances, OT, holiday), Deductions (tax, contributions, loans)
- Download as PDF

### 6.3 Salary Configuration

**Route:** `/salary`
**Roles:** Super Admin, Admin

- **Pay Grades** — Define salary bands and ranges
- **Salary Structures** — Set base pay, allowances, benefits per grade
- **Allowance/Deduction Types** — Configure earning and deduction categories

### 6.4 Loans

**Route:** `/loans`
**Roles:** All

- Apply for salary advance or company loan
- View loan balance and repayment schedule
- Automatic payroll deduction for loan repayments

### 6.5 Final Pay

**Route:** `/final-pay`
**Roles:** Super Admin, Admin

- Calculate final settlement for departing employees
- Includes: remaining salary, leave encashment, 13th month pro-rata, deductions
- Approval workflow before release

### 6.6 Expenses

**Route:** `/expenses`
**Roles:** All

- File expense reimbursement claims
- Attach receipt images or PDFs
- Track claim status (Pending, Approved, Reimbursed)

---

## 7. Approvals & Workflows

### 7.1 Approvals

**Route:** `/approvals`
**Roles:** Super Admin, Admin, Manager

- **Pending Queue** — All items awaiting your approval
- **Bulk Actions** — Approve or reject multiple requests at once
- **Types:** Leave requests, overtime requests, expense claims, loan applications
- Click any item to view details and take action

### 7.2 Workflow Rules

**Route:** `/workflow-rules`
**Roles:** Super Admin, Admin

Create automation rules for HR processes:

- **Conditions** — When leave type = Sick AND days > 3, require medical certificate
- **Actions** — Auto-approve, escalate, notify, require attachment
- **Priority** — Set rule execution order
- **SLA Configuration** — Define approval time limits

### 7.3 Workflow Triggers

**Route:** `/workflow-triggers`
**Roles:** Super Admin, Admin

Event-based automations:

- **Event Types** — Employee joined, probation ended, contract expiring, etc.
- **Trigger Actions** — Send notification, create task, update record

### 7.4 Workflow Analytics

**Route:** `/workflow-analytics`
**Roles:** Super Admin, Admin, Manager

- Execution statistics (total runs, success/failure rates)
- SLA compliance tracking
- Average approval times
- Bottleneck identification

### 7.5 Workflow Decisions

**Route:** `/workflow-decisions`
**Roles:** Super Admin, Admin, Manager

- View AI-powered routing decisions
- Override automated decisions when needed
- Decision audit trail

---

## 8. Talent Development & Performance

### 8.1 Performance Reviews

**Route:** `/performance`
**Roles:** Super Admin, Admin, Manager

- **Create Review Cycles** — Annual, semi-annual, or quarterly
- **Set Goals** — Define KPIs and targets per employee
- **Rating Scales** — Configurable rating criteria
- **360 Feedback** — Self, peer, and manager reviews
- **Review Dashboard** — Track completion rates across the organization

### 8.2 Training

**Route:** `/training`
**Roles:** All

- **Course Management** — Create and manage training programs
- **Assign Courses** — Assign to individuals or departments
- **Track Completion** — Monitor progress and certifications
- **Certificate Expiry** — Alerts for expiring certifications

### 8.3 Milestones

**Route:** `/milestones`
**Roles:** Super Admin, Admin, Manager

- **Contract Milestones** — Probation end dates, contract renewals
- **Anniversary Tracking** — Work anniversary notifications
- **Automated Alerts** — Configurable reminders before milestone dates

---

## 9. Employee Engagement & Culture

### 9.1 Announcements

**Route:** `/announcements`
**Roles:** All (Admin to create, All to view)

- Create and publish company-wide announcements
- Rich text editor with formatting
- Pin important announcements to the top

### 9.2 Recognition

**Route:** `/recognition`
**Roles:** All

- **Send Kudos** — Recognize peers for great work
- **Categories** — Teamwork, Innovation, Customer Focus, etc.
- **Recognition Feed** — Company-wide praise stream
- **Leaderboard** — Top recognized employees

### 9.3 Pulse Surveys

**Route:** `/pulse-surveys`
**Roles:** Super Admin, Admin, Manager (create); All (respond)

- **Create Surveys** — Quick 1-5 question pulse checks
- **Send to Teams** — Target specific departments or company-wide
- **Anonymous Responses** — Option for anonymous feedback
- **Results Dashboard** — Aggregate scores and trends

### 9.4 HR Requests

**Route:** `/hr-requests`
**Roles:** All

- Submit general HR inquiries and requests
- Track status from submission to resolution
- Categories: Payroll query, Benefits question, Policy clarification, etc.

### 9.5 Grievance

**Route:** `/grievance`
**Roles:** All

- File formal grievances or complaints
- Confidential handling with assigned case officers
- Track resolution progress
- Escalation workflows

### 9.6 Policies

**Route:** `/policies`
**Roles:** All

- View company policies (employee handbook, code of conduct, etc.)
- Policy acknowledgment tracking
- Version history

### 9.7 Benefits

**Route:** `/benefits`
**Roles:** All

- View enrolled benefits (health insurance, life insurance, etc.)
- Submit benefit claims
- Track claim status and history

---

## 10. Compliance & Legal

### 10.1 Compliance Dashboard

**Route:** `/compliance`
**Roles:** Super Admin, Admin

- Regulatory requirement tracking (DOLE, BIR, SSS, PhilHealth, Pag-IBIG)
- Compliance status overview with due dates
- Document submission tracking

### 10.2 Tax Filings

**Route:** `/tax-filings`
**Roles:** Super Admin, Admin

- Filing calendar for BIR, SSS, PhilHealth, Pag-IBIG submissions
- Form generation (BIR 1601C, 2316, etc.)
- Filing status tracking

### 10.3 Clearance

**Route:** `/clearance`
**Roles:** Super Admin, Admin, Manager

- Exit clearance checklists for departing employees
- Multi-department approval workflow (IT, Finance, HR, Admin)
- Track clearance completion before final pay release

### 10.4 Disciplinary

**Route:** `/disciplinary`
**Roles:** Super Admin, Admin, Manager

- Record incidents and violations
- Issue notices (verbal warning, written warning, suspension, termination)
- Track progressive discipline history
- Documentation and evidence management

### 10.5 Holidays

**Route:** `/holidays`
**Roles:** Super Admin, Admin

- **Regular Holidays** — New Year, Independence Day, Christmas, etc.
- **Special Non-Working Holidays** — EDSA, Ninoy Aquino Day, etc.
- **Company-Specific Holidays** — Foundation day, etc.
- Holidays affect payroll computation (holiday pay rates)

---

## 11. AI & Analytics

### 11.1 AI Agent Hub

**Route:** `/agent-hub`
**Roles:** All

Interact with AI agents for HR assistance:

- **HR Assistant** — General HR questions, policy lookups
- **Payroll Advisor** — Salary calculations, tax questions
- **Leave Advisor** — Leave balance queries, policy explanations
- **Available Agents** — Enable/disable agents, configure settings

**How to use:**
1. Open Agent Hub.
2. Select an AI agent (or use the default HR assistant).
3. Type your question in the chat input.
4. The AI responds with context-aware answers based on your company data.

### 11.2 Analytics

**Route:** `/analytics`
**Roles:** Super Admin, Admin

- **Headcount Trends** — Monthly employee count over time
- **Turnover Analysis** — Hire/separation rates, voluntary vs. involuntary
- **Department Cost Analysis** — Payroll cost per department
- **Leave Analytics** — Usage patterns, popular leave types, peak periods
- **Attendance Insights** — Tardiness trends, absence rates

### 11.3 Org Intelligence

**Route:** `/org-intelligence`
**Roles:** Super Admin, Admin, Manager

AI-powered organizational health insights:

- **Flight Risk Scores** — Employees likely to leave (based on patterns)
- **Burnout Risk** — Teams showing signs of burnout (overtime, absence patterns)
- **Org Health Score** — Overall organization wellness metric
- **Trend Analysis** — How metrics change over time
- **Cross-Correlations** — Relationships between different HR metrics
- **Department Comparison** — Benchmark departments against each other
- **Top Concerns** — AI-identified priority areas

---

## 12. Virtual Office

**Route:** `/virtual-office`
**Roles:** All (Admin for setup and management)

A 2D visual representation of your office with real-time employee status.

### 12.1 Setup (Admin)

1. Navigate to Virtual Office.
2. Choose a **template**: Small (up to 10), Medium (up to 30), or Large (up to 100).
3. Click **Save & Continue**.
4. Use **Auto-Assign by Department** to automatically place employees, or assign seats manually.

### 12.2 Office Canvas

- **Zoom** — Mouse wheel to zoom in/out (0.5x to 4x)
- **Pan** — Click and drag to move around the office
- **Click Seat** — Zooms to the employee and shows their info card
- **Hover Seat** — Shows a quick tooltip with name, department, and status
- **Reset Zoom** — Click the "Reset Zoom" button in the bottom-right corner

### 12.3 Employee Status

Employees appear as colored avatar circles with status rings:

| Status | Ring Color | Description |
|--------|-----------|-------------|
| Working | Green | Currently clocked in |
| Overtime | Orange | Working past regular hours |
| Focused | Blue (pulsing) | In focus mode |
| In Meeting | Purple | In a meeting room |
| On Break | Orange | Taking a break |
| Away | Gray | Temporarily away |
| On Leave | Red | On approved leave |
| Offline | Light Gray | Not active |

### 12.4 Search & Filter

Use the filter bar at the top to find employees:

- **Name Search** — Type to search by employee name
- **Department Filter** — Select a department from the dropdown
- **Status Filter** — Filter by current status

Non-matching employees are dimmed (not hidden) so you can see the full office layout.

### 12.5 Set Your Status

In the **Status Bar** at the bottom:

1. **Choose a Status** — Select from focused, in meeting, on break, away.
2. **Add Custom Status** — Type what you're working on.
3. **Add Emoji** — Click the emoji button to choose an emoji badge.
4. **Meeting Room** — If "In Meeting," select the meeting room zone.
5. Click **Set Status** to save.

### 12.6 Customize Avatar

1. Click the **avatar button** in the Status Bar.
2. Choose an **avatar type**: 6 people styles or 6 animals (cat, dog, rabbit, bear, penguin, shiba).
3. Choose a **color** from 16 presets or enter a custom hex code.
4. Click **Save**.

### 12.7 Admin: Seat Management

- **Assign Seat** — Click any empty seat dot (enlarged in admin mode) to open the assignment modal. Select an unassigned employee and confirm.
- **Remove Seat** — Click an occupied seat, then click **Remove** in the info card.
- **Auto-Assign** — Expand the setup panel and click **Auto-Assign by Department**.

### 12.8 MiniMap

A small overview map in the sidebar shows the full office layout with colored dots for each employee. Useful for navigating large offices.

---

## 13. System Administration

### 13.1 Departments

**Route:** `/departments`
**Roles:** Super Admin, Admin

- Create, edit, and delete departments
- Assign department managers
- Set department hierarchy (parent-child)

### 13.2 Positions

**Route:** `/positions`
**Roles:** Super Admin, Admin

- Create and manage job positions
- Associate positions with salary grades
- Set position descriptions

### 13.3 Users

**Route:** `/users`
**Roles:** Super Admin, Admin

- Create user accounts
- Assign roles (Super Admin, Admin, Manager, Employee)
- Enable/disable accounts
- Reset passwords

### 13.4 Settings

**Route:** `/settings`
**Roles:** Super Admin, Admin

- **Company Settings** — Company name, logo, address, TIN
- **System Preferences** — Date format, currency, timezone
- **Notification Settings** — Email notification preferences
- **Security Settings** — Password policies, session timeout

### 13.5 Audit Trail

**Route:** `/audit`
**Roles:** Super Admin, Admin

- View all system activity logs
- Filter by user, action type, date range
- Track data changes (who changed what, when)

### 13.6 Import / Export

**Route:** `/import-export`
**Roles:** Super Admin, Admin

- **Import** — Bulk upload employees, attendance, or other data via CSV
- **Export** — Download data to CSV or Excel
- **Templates** — Download import templates with required column headers

---

## 14. Integrations

**Route:** `/integrations`
**Roles:** Super Admin, Admin

### 14.1 Available Integrations

- **Slack** — Send HR notifications to Slack channels
- **GitHub** — Sync developer profiles and activity
- **Google Workspace** — Directory sync and calendar integration
- **Telegram Bot** — Employee self-service via Telegram
- **AIStarlight Accounting** — Sync payroll data to accounting system

### 14.2 Setting Up an Integration

1. Go to **Integrations**.
2. Click on the integration you want to connect.
3. Follow the setup instructions (provide API keys, OAuth, etc.).
4. Test the connection.
5. Enable the integration.

### 14.3 Provisioning Jobs

**Route:** `/integrations/jobs`

- Monitor automated provisioning jobs (e.g., creating Google accounts for new hires)
- View job history and logs
- Schedule recurring provisioning tasks

---

## 15. Account & Billing

### 15.1 Billing

**Route:** `/billing`
**Roles:** Super Admin, Admin

- View current subscription plan
- Manage payment methods
- View invoice history
- Upgrade/downgrade plan

### 15.2 Referrals

**Route:** `/referrals`
**Roles:** Super Admin, Admin

- Invite other companies to HalaOS
- Track referral status
- Earn referral rewards

---

## Appendix: Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `/` | Focus search bar |
| `Esc` | Close modal/popup |

## Appendix: Language Support

HalaOS supports:
- **English** — Default
- **Chinese (中文)** — Toggle via profile settings

Switch language from the **Profile** page or the top navigation bar.

---

*For technical support, contact your system administrator or reach out via the AI Agent Hub.*
