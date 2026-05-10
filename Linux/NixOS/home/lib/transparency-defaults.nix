{
  shell = 0.8;
  content = 0.2;
  vendorShell = 0.2;
  perShell = {
    caelestia = {
      shell = 0.8;
      content = 0.2;
    };
    noctalia = {
      shell = 0.8;
      content = 0.2;
    };
    # end4 QML window itself is the "shell" surface, not an overlay - lower
    # opacity here is intentional and matches upstream end-4 expectations.
    end4 = {
      shell = 0.2;
      content = 0.2;
      regionContent = 0.2;
    };
  };
}
