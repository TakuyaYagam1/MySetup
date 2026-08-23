//@ pragma DefaultEnv QSG_RENDER_LOOP=threaded

import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Wayland
import "selector-model.js" as SelectorModel

ShellRoot {
  id: root

  readonly property string targetScreen: Quickshell.env("WAHRWELT_SHELL_SELECTOR_MONITOR") || ""
  readonly property string activeShell: Quickshell.env("WAHRWELT_ACTIVE_SHELL") || "caelestia"
  readonly property string rememberedEnd4Variant: Quickshell.env("WAHRWELT_END4_VARIANT") || "end4"
  readonly property string selectorScript: Quickshell.env("WAHRWELT_SHELL_SELECTOR_SCRIPT")

  property var shellOptions: [
    // WAHRWELT_SHELL_OPTIONS_BEGIN
    {
      id: "noctalia",
      family: "noctalia",
      title: "noctalia",
      accent: "#f48ab6",
      surface: "#211f31",
      quickshellConfig: "",
      variantLabel: "",
      logo: Qt.resolvedUrl("assets/noctalia.svg")
    },
    {
      id: "caelestia",
      family: "caelestia",
      title: "caelestia-shell",
      accent: "#6ae5e1",
      surface: "#201f33",
      quickshellConfig: "",
      variantLabel: "",
      logo: Qt.resolvedUrl("assets/caelestia.svg")
    },
    {
      id: "end4",
      family: "end4",
      title: "end-4",
      accent: "#51debd",
      surface: "#1d2122",
      quickshellConfig: "ii",
      variantLabel: "Official",
      logo: Qt.resolvedUrl("assets/illogical-impulse.svg")
    },
    {
      id: "end4-pc",
      family: "end4",
      title: "end-4",
      accent: "#51debd",
      surface: "#1d2122",
      quickshellConfig: "end4-pC",
      variantLabel: "pC",
      logo: Qt.resolvedUrl("assets/illogical-impulse.svg")
    }
    // WAHRWELT_SHELL_OPTIONS_END
  ]

  readonly property var cardOptions: SelectorModel.buildCards(shellOptions)
  readonly property string activeShellFamily: SelectorModel.activeFamily(shellOptions, activeShell)
  readonly property string selectedEnd4Variant: SelectorModel.end4Target(activeShell, rememberedEnd4Variant)

  readonly property var visibleScreens: {
    const screens = Quickshell.screens || [];
    if (!targetScreen) {
      return screens.length > 0 ? screens.slice(0, 1) : [];
    }

    const matches = screens.filter(screen => screen.name === targetScreen);
    if (matches.length > 0) {
      return matches;
    }

    return screens.length > 0 ? screens.slice(0, 1) : [];
  }

  function shellIndex(shellId) {
    const family = SelectorModel.activeFamily(shellOptions, shellId);
    for (let i = 0; i < cardOptions.length; i++) {
      if (cardOptions[i].family === family) {
        return i;
      }
    }
    return 0;
  }

  function switchShell(shellId) {
    console.log("[selector] activate", shellId, "via", selectorScript);
    Quickshell.execDetached(["bash", selectorScript, "switch", shellId]);
    Qt.callLater(() => Qt.quit());
  }

  Variants {
    model: root.visibleScreens

    delegate: PanelWindow {
      id: window

      required property ShellScreen modelData

      screen: modelData
      visible: true
      color: "transparent"

      anchors.top: true
      anchors.left: true
      anchors.right: true
      anchors.bottom: true

      WlrLayershell.layer: WlrLayer.Overlay
      WlrLayershell.keyboardFocus: WlrKeyboardFocus.Exclusive
      WlrLayershell.namespace: "wahrwelt-shell-selector-" + modelData.name
      WlrLayershell.exclusionMode: ExclusionMode.Ignore

      property int selectedIndex: root.shellIndex(root.activeShell)

      function closeOverlay() {
        Qt.quit();
      }

      function moveSelection(delta) {
        const count = root.cardOptions.length;
        if (count === 0) {
          return;
        }
        selectedIndex = ((selectedIndex + delta) % count + count) % count;
      }

      function activateSelection(index) {
        const card = root.cardOptions[index];
        if (!card) {
          return;
        }
        root.switchShell(SelectorModel.cardTarget(card, root.activeShell, root.rememberedEnd4Variant));
      }

      Item {
        id: overlay
        anchors.fill: parent
        focus: true

        Component.onCompleted: Qt.callLater(() => overlay.forceActiveFocus())

        Keys.onPressed: event => {
                          switch (event.key) {
                          case Qt.Key_Escape:
                            window.closeOverlay();
                            event.accepted = true;
                            return;
                          case Qt.Key_Left:
                            window.moveSelection(-1);
                            event.accepted = true;
                            return;
                          case Qt.Key_Right:
                            window.moveSelection(1);
                            event.accepted = true;
                            return;
                          case Qt.Key_Return:
                          case Qt.Key_Enter:
                            window.activateSelection(window.selectedIndex);
                            event.accepted = true;
                            return;
                          case Qt.Key_1:
                            window.activateSelection(0);
                            event.accepted = true;
                            return;
                          case Qt.Key_2:
                            window.activateSelection(1);
                            event.accepted = true;
                            return;
                          case Qt.Key_3:
                            window.activateSelection(2);
                            event.accepted = true;
                            return;
                          }
                        }

        Rectangle {
          anchors.fill: parent
          color: "#0f1320"
          opacity: 0.54
        }

        Rectangle {
          width: Math.round(parent.width * 0.34)
          height: width
          radius: width / 2
          color: "#5b6fe7"
          opacity: 0.10
          anchors.left: parent.left
          anchors.leftMargin: -Math.round(width * 0.2)
          anchors.bottom: parent.bottom
          anchors.bottomMargin: -Math.round(height * 0.35)
        }

        Rectangle {
          width: Math.round(parent.width * 0.24)
          height: width
          radius: width / 2
          color: "#f48ab6"
          opacity: 0.08
          anchors.horizontalCenter: parent.horizontalCenter
          anchors.verticalCenter: parent.verticalCenter
          anchors.verticalCenterOffset: Math.round(parent.height * 0.12)
        }

        MouseArea {
          anchors.fill: parent
          acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
          onClicked: window.closeOverlay()
        }

        Item {
          id: cardGroup
          anchors.centerIn: parent
          width: buttonFlow.implicitWidth
          height: buttonFlow.implicitHeight

          Flow {
            id: buttonFlow
            anchors.centerIn: parent
            spacing: 24

            Repeater {
              model: root.cardOptions

              delegate: Rectangle {
                id: card

                readonly property var cardData: modelData
                readonly property bool isSelected: window.selectedIndex === index
                readonly property bool isHovered: hoverHandler.hovered
                readonly property bool isActiveShell: root.activeShellFamily === cardData.family
                property real cardScale: (isSelected || isHovered) ? 1.045 : 1.0

                width: 238
                height: 236
                radius: 28
                color: {
                  if (isSelected || isHovered) {
                    return Qt.tint(cardData.surface, Qt.rgba(1, 1, 1, 0.08));
                  }
                  return cardData.surface;
                }
                border.width: isSelected || isActiveShell ? 2 : 1
                border.color: isSelected || isActiveShell ? cardData.accent : Qt.rgba(0.55, 0.62, 0.83, 0.42)

                layer.enabled: true
                layer.smooth: true
                transform: Scale {
                  origin.x: card.width / 2
                  origin.y: card.height / 2
                  xScale: card.cardScale
                  yScale: card.cardScale
                }

                Behavior on color {
                  ColorAnimation {
                    duration: 140
                    easing.type: Easing.OutCubic
                  }
                }

                Behavior on border.color {
                  ColorAnimation {
                    duration: 140
                    easing.type: Easing.OutCubic
                  }
                }

                Behavior on cardScale {
                  NumberAnimation {
                    duration: 180
                    easing.type: Easing.OutBack
                    easing.overshoot: 0.45
                  }
                }

                MouseArea {
                  anchors.fill: parent
                  hoverEnabled: true
                  cursorShape: Qt.PointingHandCursor
                  onEntered: window.selectedIndex = index
                  onClicked: root.switchShell(SelectorModel.cardTarget(card.cardData, root.activeShell, root.rememberedEnd4Variant))
                }

                Rectangle {
                  anchors.top: parent.top
                  anchors.right: parent.right
                  anchors.margins: 12
                  width: 34
                  height: 34
                  radius: 17
                  color: (card.isSelected || card.isHovered) ? "#edf1ff" : Qt.rgba(0.20, 0.23, 0.37, 0.92)
                  border.width: 1
                  border.color: (card.isSelected || card.isHovered) ? "#edf1ff" : Qt.rgba(0.54, 0.58, 0.79, 0.55)

                  Text {
                    anchors.centerIn: parent
                    text: String(index + 1)
                    color: (card.isSelected || card.isHovered) ? card.cardData.accent : "#d9e1ff"
                    font.pixelSize: 14
                    font.family: "monospace"
                    font.weight: Font.Medium
                  }
                }

                ColumnLayout {
                  anchors.fill: parent
                  anchors.margins: 24
                  spacing: 14

                  Item {
                    Layout.fillHeight: true
                  }

                  Image {
                    Layout.alignment: Qt.AlignHCenter
                    source: card.cardData.logo
                    fillMode: Image.PreserveAspectFit
                    sourceSize.width: 112
                    sourceSize.height: 112
                    width: 112
                    height: 82
                    smooth: true
                    mipmap: true
                  }

                  Text {
                    Layout.alignment: Qt.AlignHCenter
                    text: card.cardData.title
                    color: card.isSelected || card.isHovered ? "#f4f7ff" : "#d9e1ff"
                    font.pixelSize: 18
                    font.family: "monospace"
                    font.weight: Font.Medium
                    horizontalAlignment: Text.AlignHCenter
                  }

                  Item {
                    id: variantControl

                    Layout.alignment: Qt.AlignHCenter
                    implicitWidth: 174
                    implicitHeight: 34
                    visible: card.cardData.family === "end4"

                    Rectangle {
                      anchors.fill: parent
                      radius: 12
                      color: Qt.rgba(0.08, 0.10, 0.16, 0.72)
                      border.width: 1
                      border.color: Qt.rgba(0.62, 0.70, 0.88, 0.32)
                    }

                    Rectangle {
                      width: (variantControl.width - 4) / 2
                      height: variantControl.height - 4
                      x: root.selectedEnd4Variant === "end4-pc" ? variantControl.width / 2 : 2
                      y: 2
                      radius: 10
                      color: card.cardData.accent
                      opacity: 0.88

                      Behavior on x {
                        NumberAnimation {
                          duration: 180
                          easing.type: Easing.OutCubic
                        }
                      }
                    }

                    Row {
                      anchors.fill: parent
                      anchors.margins: 2

                      Repeater {
                        model: card.cardData.variants

                        delegate: Item {
                          width: (variantControl.width - 4) / Math.max(1, card.cardData.variants.length)
                          height: variantControl.height - 4

                          Text {
                            anchors.centerIn: parent
                            text: modelData.variantLabel || modelData.id
                            color: root.selectedEnd4Variant === modelData.id ? "#10161a" : "#d9e1ff"
                            font.pixelSize: 12
                            font.family: "monospace"
                            font.weight: Font.DemiBold
                          }

                          MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            preventStealing: true
                            onClicked: root.switchShell(modelData.id)
                          }
                        }
                      }
                    }
                  }

                  Rectangle {
                    Layout.alignment: Qt.AlignHCenter
                    Layout.topMargin: 2
                    width: card.isActiveShell ? 74 : 0
                    height: 3
                    radius: 2
                    color: card.cardData.accent
                    opacity: 0.95

                    Behavior on width {
                      NumberAnimation {
                        duration: 140
                        easing.type: Easing.OutCubic
                      }
                    }
                  }

                  Item {
                    Layout.fillHeight: true
                  }
                }

                HoverHandler {
                  id: hoverHandler
                  onHoveredChanged: {
                    if (hovered) {
                      window.selectedIndex = index;
                    }
                  }
                }

              }
            }
          }
        }
      }
    }
  }
}
