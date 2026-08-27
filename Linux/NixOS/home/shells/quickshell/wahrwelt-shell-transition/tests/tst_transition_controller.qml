import QtQuick
import QtTest
import ".."

TestCase {
  id: testCase
  name: "TransitionController"

  property var controller: null

  Component {
    id: controllerComponent
    TransitionController {}
  }

  SignalSpy {
    id: exitSpy
    signalName: "exitRequested"
  }

  function init() {
    controller = createTemporaryObject(controllerComponent, testCase);
    verify(controller !== null);
    exitSpy.target = controller;
    exitSpy.clear();
  }

  function cleanup() {
    exitSpy.target = null;
    controller.destroy();
    controller = null;
  }

  function test_watchdogSignalFailsOpenWithoutClockInput() {
    controller.initialize(["DP-1"]);
    compare(controller.state, "capturing");
    compare(controller.watchdogInterval, 5000);
    verify(controller.watchdogRunning);

    controller.phaseWatchdog.triggered();
    compare(controller.state, "aborted");
    verify(!controller.watchdogRunning);
    compare(exitSpy.count, 1);
  }

  function test_revealGetsFreshCleanupDeadline() {
    controller.initialize(["DP-1"]);
    controller.captureReady("DP-1");
    compare(controller.state, "captured");
    compare(controller.watchdogInterval, 30000);

    verify(controller.reveal());
    compare(controller.state, "revealing");
    compare(controller.watchdogInterval, 6000);
    verify(controller.watchdogRunning);
    compare(exitSpy.count, 0);

    controller.completeReveal();
    compare(controller.state, "done");
    verify(!controller.watchdogRunning);
    compare(exitSpy.count, 1);
  }

  function test_explicitWatchdogExpiryAbortsCapturedPhase() {
    controller.initialize(["DP-1"]);
    controller.captureReady("DP-1");
    compare(controller.state, "captured");

    verify(controller.expireWatchdog());
    compare(controller.state, "aborted");
    compare(exitSpy.count, 1);
  }

  function test_frameLossAfterCaptureFailsOpen() {
    controller.initialize(["DP-1"]);
    controller.captureReady("DP-1");
    compare(controller.state, "captured");

    verify(controller.captureFailed("DP-1"));
    compare(controller.state, "aborted");
    verify(!controller.watchdogRunning);
    compare(exitSpy.count, 1);
  }

  function test_frameLossDuringRevealFailsOpen() {
    controller.initialize(["DP-1"]);
    controller.captureReady("DP-1");
    verify(controller.reveal());
    compare(controller.state, "revealing");

    verify(controller.captureFailed("DP-1"));
    compare(controller.state, "aborted");
    verify(!controller.watchdogRunning);
    compare(exitSpy.count, 1);
  }

  function test_allScreensMustCaptureBeforeReveal() {
    controller.initialize(["DP-1", "HDMI-A-1"]);
    controller.captureReady("DP-1");
    compare(controller.state, "capturing");
    verify(!controller.reveal());

    controller.captureReady("HDMI-A-1");
    compare(controller.state, "captured");
    verify(controller.reveal());
    compare(controller.state, "revealing");
  }

  function test_screenHotplugAborts() {
    controller.initialize(["DP-1", "HDMI-A-1"]);

    verify(!controller.checkScreens(["DP-1"]));
    compare(controller.state, "aborted");
    verify(!controller.watchdogRunning);
    compare(exitSpy.count, 1);
  }
}
