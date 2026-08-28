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

  function test_captureWatchdogFailsOpenWithoutClockInput() {
    controller.initialize(["DP-1"]);
    compare(controller.state, "capturing");
    compare(controller.captureWatchdogInterval, 5000);
    verify(controller.captureWatchdogRunning);

    controller.captureWatchdog.triggered();
    compare(controller.state, "aborted");
    verify(!controller.captureWatchdogRunning);
    verify(!controller.masterWatchdogRunning);
    compare(exitSpy.count, 1);
  }

  function test_oneMasterWatchdogCoversEveryVisiblePhase() {
    controller.initialize(["DP-1"]);
    controller.captureReady("DP-1");
    compare(controller.state, "captured");
    compare(controller.captureWatchdogInterval, 1000);
    verify(controller.captureWatchdogRunning);

    verify(controller.beginTransition());
    compare(controller.state, "outgoing");
    compare(controller.masterWatchdogInterval, 10750);
    verify(!controller.captureWatchdogRunning);
    verify(controller.masterWatchdogRunning);
    compare(exitSpy.count, 0);

    verify(controller.coverFramePresented("DP-1"));
    compare(controller.state, "outgoing");
    verify(controller.coverFramePresented("DP-1"));
    compare(controller.state, "covered");
    verify(controller.masterWatchdogRunning);

    verify(controller.beginIncoming());
    compare(controller.state, "incoming");
    verify(controller.masterWatchdogRunning);

    verify(controller.beginSettling());
    compare(controller.state, "settling");
    verify(controller.masterWatchdogRunning);

    verify(controller.settlingFramePresented("DP-1"));
    compare(controller.state, "settling");
    verify(controller.settlingFramePresented("DP-1"));
    compare(controller.state, "done");
    verify(!controller.masterWatchdogRunning);
    compare(exitSpy.count, 0);
  }

  function test_masterWatchdogExpiryAbortsVisibleTransition() {
    controller.initialize(["DP-1"]);
    controller.captureReady("DP-1");
    verify(controller.beginTransition());
    verify(controller.coverFramePresented("DP-1"));
    compare(controller.state, "outgoing");
    verify(controller.coverFramePresented("DP-1"));
    compare(controller.state, "covered");

    controller.masterWatchdog.triggered();
    compare(controller.state, "aborted");
    verify(!controller.masterWatchdogRunning);
    compare(exitSpy.count, 1);
  }

  function test_frameLossAfterCaptureFailsOpen() {
    controller.initialize(["DP-1"]);
    controller.captureReady("DP-1");
    compare(controller.state, "captured");

    verify(controller.captureFailed("DP-1"));
    compare(controller.state, "aborted");
    verify(!controller.captureWatchdogRunning);
    verify(!controller.masterWatchdogRunning);
    compare(exitSpy.count, 1);
  }

  function test_frameLossDuringOutgoingFailsOpen() {
    controller.initialize(["DP-1"]);
    controller.captureReady("DP-1");
    verify(controller.beginTransition());
    compare(controller.state, "outgoing");

    verify(controller.captureFailed("DP-1"));
    compare(controller.state, "aborted");
    verify(!controller.captureWatchdogRunning);
    verify(!controller.masterWatchdogRunning);
    compare(exitSpy.count, 1);
  }

  function test_allScreensMustCaptureBeforeOutgoing() {
    controller.initialize(["DP-1", "HDMI-A-1"]);
    controller.captureReady("DP-1");
    compare(controller.state, "capturing");
    verify(!controller.beginTransition());

    controller.captureReady("HDMI-A-1");
    compare(controller.state, "captured");
    verify(controller.beginTransition());
    compare(controller.state, "outgoing");
  }

  function test_allScreensMustPresentOpaqueCoverBeforeCovered() {
    controller.initialize(["DP-1", "HDMI-A-1"]);
    controller.captureReady("DP-1");
    controller.captureReady("HDMI-A-1");
    verify(controller.beginTransition());

    verify(controller.coverFramePresented("DP-1"));
    compare(controller.state, "outgoing");
    verify(controller.coverFramePresented("DP-1"));
    compare(controller.state, "outgoing");
    verify(controller.coverFramePresented("HDMI-A-1"));
    compare(controller.state, "outgoing");
    verify(controller.coverFramePresented("HDMI-A-1"));
    compare(controller.state, "covered");
  }

  function test_unknownCaptureScreenAbortsAndExits() {
    controller.initialize(["DP-1"]);
    controller.captureReady("HDMI-A-1");

    compare(controller.state, "aborted");
    compare(exitSpy.count, 1);
  }

  function test_unknownCoverScreenAbortsAndExits() {
    controller.initialize(["DP-1"]);
    controller.captureReady("DP-1");
    verify(controller.beginTransition());

    verify(!controller.coverFramePresented("HDMI-A-1"));
    compare(controller.state, "aborted");
    compare(exitSpy.count, 1);
  }

  function test_unknownSettlingScreenAbortsAndExits() {
    controller.initialize(["DP-1"]);
    controller.captureReady("DP-1");
    verify(controller.beginTransition());
    verify(controller.coverFramePresented("DP-1"));
    verify(controller.coverFramePresented("DP-1"));
    verify(controller.beginIncoming());
    verify(controller.beginSettling());

    verify(!controller.settlingFramePresented("HDMI-A-1"));
    compare(controller.state, "aborted");
    compare(exitSpy.count, 1);
  }

  function test_screenHotplugAborts() {
    controller.initialize(["DP-1", "HDMI-A-1"]);

    verify(!controller.checkScreens(["DP-1"]));
    compare(controller.state, "aborted");
    verify(!controller.captureWatchdogRunning);
    verify(!controller.masterWatchdogRunning);
    compare(exitSpy.count, 1);
  }
}
