import { driver, type DriveStep } from "driver.js";
import "driver.js/dist/driver.css";
import { useAuthStore } from "../stores/auth";
import { useI18n } from "vue-i18n";

const TOUR_DONE_KEY = "aigonhr_tour_done";

export function useTour() {
  const auth = useAuthStore();
  const { t } = useI18n();

  function hasDoneTour(): boolean {
    return localStorage.getItem(TOUR_DONE_KEY) === "true";
  }

  function markTourDone() {
    localStorage.setItem(TOUR_DONE_KEY, "true");
  }

  function resetTour() {
    localStorage.removeItem(TOUR_DONE_KEY);
  }

  function getAdminSteps(): DriveStep[] {
    return [
      {
        element: "#app-logo",
        popover: {
          title: t("tour.welcomeTitle"),
          description: t("tour.welcomeDesc"),
        },
      },
      {
        element: "#sidebar-menu",
        popover: {
          title: t("tour.sidebarTitle"),
          description: t("tour.sidebarDesc"),
        },
      },
      {
        element: "#dashboard-stats",
        popover: {
          title: t("tour.statsTitle"),
          description: t("tour.statsDesc"),
        },
      },
      {
        element: "#dashboard-clock",
        popover: {
          title: t("tour.clockTitle"),
          description: t("tour.clockDesc"),
        },
      },
      {
        element: "#dashboard-briefing",
        popover: {
          title: t("tour.briefingTitle"),
          description: t("tour.briefingDesc"),
        },
      },
      {
        element: "#chat-panel-trigger",
        popover: {
          title: t("tour.chatTitle"),
          description: t("tour.chatDesc"),
        },
      },
      {
        element: "#header-notifications",
        popover: {
          title: t("tour.notifTitle"),
          description: t("tour.notifDesc"),
        },
      },
      {
        element: "#header-user",
        popover: {
          title: t("tour.userTitle"),
          description: t("tour.userDesc"),
        },
      },
    ];
  }

  function getEmployeeSteps(): DriveStep[] {
    return [
      {
        element: "#app-logo",
        popover: {
          title: t("tour.welcomeTitle"),
          description: t("tour.empWelcomeDesc"),
        },
      },
      {
        element: "#sidebar-menu",
        popover: {
          title: t("tour.sidebarTitle"),
          description: t("tour.empSidebarDesc"),
        },
      },
      {
        element: "#dashboard-clock",
        popover: {
          title: t("tour.clockTitle"),
          description: t("tour.empClockDesc"),
        },
      },
      {
        element: "#chat-panel-trigger",
        popover: {
          title: t("tour.chatTitle"),
          description: t("tour.empChatDesc"),
        },
      },
      {
        element: "#header-notifications",
        popover: {
          title: t("tour.notifTitle"),
          description: t("tour.empNotifDesc"),
        },
      },
      {
        element: "#header-user",
        popover: {
          title: t("tour.userTitle"),
          description: t("tour.empUserDesc"),
        },
      },
    ];
  }

  function startTour() {
    const steps = auth.isAdmin ? getAdminSteps() : getEmployeeSteps();
    const d = driver({
      showProgress: true,
      animate: true,
      allowClose: true,
      stagePadding: 4,
      stageRadius: 8,
      nextBtnText: t("tour.next"),
      prevBtnText: t("tour.prev"),
      doneBtnText: t("tour.done"),
      progressText: "{{current}} / {{total}}",
      onDestroyStarted: () => {
        markTourDone();
        d.destroy();
      },
      steps,
    });
    d.drive();
  }

  function autoStartIfNeeded() {
    if (!hasDoneTour()) {
      setTimeout(() => startTour(), 800);
    }
  }

  return { startTour, autoStartIfNeeded, resetTour, hasDoneTour };
}
