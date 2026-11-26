{
  description = "Ink language compiler";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      each = f: nixpkgs.lib.genAttrs systems (system: f system nixpkgs.legacyPackages.${system});
    in
    {
      packages = each (
        system: pkgs: {
          ink = pkgs.buildGoModule {
            pname = "ink";
            version = "0.3.0";
            src = self;
            subPackages = [ "cmd/ink" ];
            vendorHash = "sha256-iQ0yy5jHdTLDUauS806a/hDkKLv4KQUltfPEo/Ma2v8=";
            ldflags = [ "-s" "-w" ];
            meta = {
              description = "Ink language compiler";
              mainProgram = "ink";
              platforms = systems;
            };
          };
          default = self.packages.${system}.ink;
        }
      );

      devShells = each (
        system: pkgs: {
          default = pkgs.mkShell {
            packages = [
              self.packages.${system}.ink
              pkgs.go
            ];
          };
        }
      );

      apps = each (
        system: _: {
          default = {
            type = "app";
            program = "${self.packages.${system}.ink}/bin/ink";
          };
        }
      );
    };
}
