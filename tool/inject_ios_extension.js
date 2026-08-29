// Injects the PacketTunnel extension target into ios/Runner.xcodeproj/project.pbxproj.
// Idempotent: exits early if the target already exists.
const fs = require('fs');
const path = require('path');

const projectPath = path.join(__dirname, '..', 'ios', 'Runner.xcodeproj', 'project.pbxproj');
let p = fs.readFileSync(projectPath, 'utf8');
if (p.includes('PacketTunnel')) {
  console.log('PacketTunnel target already present, skipping');
  process.exit(0);
}

const A = 'F1000000000000000000000A'; // build file: PacketTunnelProvider.swift in Sources
const B = 'F1000000000000000000000B'; // build file: libclash.a in Frameworks
const F1 = 'F100000000000000000000F1'; // fileref PacketTunnelProvider.swift
const F2 = 'F100000000000000000000F2'; // fileref PacketTunnel/Info.plist
const F3 = 'F100000000000000000000F3'; // fileref PacketTunnel.entitlements
const F4 = 'F100000000000000000000F4'; // fileref Runner.entitlements
const F5 = 'F100000000000000000000F5'; // fileref PacketTunnel-Bridging-Header.h
const F6 = 'F100000000000000000000F6'; // fileref libclash.a
const G1 = 'F100000000000000000000G1'; // group PacketTunnel
const T1 = 'F100000000000000000000T1'; // native target PacketTunnel
const P1 = 'F100000000000000000000P1'; // product PacketTunnel.appex
const S1 = 'F100000000000000000000S1'; // sources phase ext
const W1 = 'F100000000000000000000W1'; // frameworks phase ext
const E1 = 'F100000000000000000000E1'; // embed foundation extensions phase (Runner)
const X1 = 'F100000000000000000000X1'; // container item proxy
const D1 = 'F100000000000000000000D1'; // target dependency
const C1 = 'F100000000000000000000C1'; // ext Debug
const C2 = 'F100000000000000000000C2'; // ext Release
const C3 = 'F100000000000000000000C3'; // ext Profile
const L1 = 'F100000000000000000000L1'; // ext config list

// 1. PBXBuildFile
p = p.replace('/* Begin PBXBuildFile section */',
`/* Begin PBXBuildFile section */
		${A} /* PacketTunnelProvider.swift in Sources */ = {isa = PBXBuildFile; fileRef = ${F1} /* PacketTunnelProvider.swift */; };
		${B} /* libclash.a in Frameworks */ = {isa = PBXBuildFile; fileRef = ${F6} /* libclash.a */; };`);

// 2. PBXFileReference
p = p.replace('/* End PBXFileReference section */',
`		${F1} /* PacketTunnelProvider.swift */ = {isa = PBXFileReference; lastKnownFileType = sourcecode.swift; path = PacketTunnelProvider.swift; sourceTree = "<group>"; };
		${F2} /* Info.plist */ = {isa = PBXFileReference; lastKnownFileType = text.plist.xml; path = Info.plist; sourceTree = "<group>"; };
		${F3} /* PacketTunnel.entitlements */ = {isa = PBXFileReference; lastKnownFileType = text.plist.entitlements; path = PacketTunnel.entitlements; sourceTree = "<group>"; };
		${F4} /* Runner.entitlements */ = {isa = PBXFileReference; lastKnownFileType = text.plist.entitlements; path = Runner.entitlements; sourceTree = "<group>"; };
		${F5} /* PacketTunnel-Bridging-Header.h */ = {isa = PBXFileReference; lastKnownFileType = sourcecode.c.h; path = "PacketTunnel-Bridging-Header.h"; sourceTree = "<group>"; };
		${F6} /* libclash.a */ = {isa = PBXFileReference; lastKnownFileType = archive.ar; path = Libs/libclash.a; sourceTree = "<group>"; };
		${P1} /* PacketTunnel.appex */ = {isa = PBXFileReference; explicitFileType = "wrapper.app-extension"; includeInIndex = 0; path = PacketTunnel.appex; sourceTree = BUILT_PRODUCTS_DIR; };
/* End PBXFileReference section */`);

// 3. Embed Foundation Extensions copy phase
p = p.replace('/* End PBXCopyFilesBuildPhase section */',
`		${E1} /* Embed Foundation Extensions */ = {
			isa = PBXCopyFilesBuildPhase;
			buildActionMask = 2147483647;
			dstPath = "";
			dstSubfolderSpec = 13;
			files = (
				${P1} /* PacketTunnel.appex */,
			);
			name = "Embed Foundation Extensions";
			runOnlyForDeploymentPostprocessing = 0;
		};
/* End PBXCopyFilesBuildPhase section */`);

// 4. Frameworks phase for extension
p = p.replace('/* End PBXFrameworksBuildPhase section */',
`		${W1} /* Frameworks */ = {
			isa = PBXFrameworksBuildPhase;
			buildActionMask = 2147483647;
			files = (
				${B} /* libclash.a in Frameworks */,
			);
			runOnlyForDeploymentPostprocessing = 0;
		};
/* End PBXFrameworksBuildPhase section */`);

// 5. Groups: PacketTunnel group + Products entry + Runner entitlements ref
p = p.replace(`		97C146EF1CF9000F007C117D /* Products */ = {
			isa = PBXGroup;
			children = (
				97C146EE1CF9000F007C117D /* Runner.app */,
				331C8081294A63A400263BE5 /* RunnerTests.xctest */,
			);`,
`		${G1} /* PacketTunnel */ = {
			isa = PBXGroup;
			children = (
				${F1} /* PacketTunnelProvider.swift */,
				${F5} /* PacketTunnel-Bridging-Header.h */,
				${F3} /* PacketTunnel.entitlements */,
				${F2} /* Info.plist */,
				${F6} /* libclash.a */,
			);
			path = PacketTunnel;
			sourceTree = "<group>";
		};
		97C146EF1CF9000F007C117D /* Products */ = {
			isa = PBXGroup;
			children = (
				97C146EE1CF9000F007C117D /* Runner.app */,
				${P1} /* PacketTunnel.appex */,
				331C8081294A63A400263BE5 /* RunnerTests.xctest */,
			);`);

p = p.replace(`			children = (
				9740EEB11CF90186004384FC /* Flutter */,
				97C146F01CF9000F007C117D /* Runner */,
				97C146EF1CF9000F007C117D /* Products */,
				331C8082294A63A400263BE5 /* RunnerTests */,
			);`,
`			children = (
				9740EEB11CF90186004384FC /* Flutter */,
				97C146F01CF9000F007C117D /* Runner */,
				${G1} /* PacketTunnel */,
				97C146EF1CF9000F007C117D /* Products */,
				331C8082294A63A400263BE5 /* RunnerTests */,
			);`);

p = p.replace(`				97C147021CF9000F007C117D /* Info.plist */,
				1498D2321E8E86230040F4C2 /* GeneratedPluginRegistrant.h */,`,
`				97C147021CF9000F007C117D /* Info.plist */,
				${F4} /* Runner.entitlements */,
				1498D2321E8E86230040F4C2 /* GeneratedPluginRegistrant.h */,`);

// 6. Native target for PacketTunnel + Runner embed phase + dependency
p = p.replace(`			productType = "com.apple.product-type.application";
		};
/* End PBXNativeTarget section */`,
`			productType = "com.apple.product-type.application";
		};
		${T1} /* PacketTunnel */ = {
			isa = PBXNativeTarget;
			buildConfigurationList = ${L1} /* Build configuration list for PBXNativeTarget "PacketTunnel" */;
			buildPhases = (
				${S1} /* Sources */,
				${W1} /* Frameworks */,
			);
			buildRules = (
			);
			dependencies = (
			);
			name = PacketTunnel;
			packageProductDependencies = (
			);
			productName = PacketTunnel;
			productReference = ${P1} /* PacketTunnel.appex */;
			productType = "com.apple.product-type.app-extension";
		};
/* End PBXNativeTarget section */`);

p = p.replace(`			buildPhases = (
				9740EEB61CF901F6004384FC /* Run Script */,
				97C146EA1CF9000F007C117D /* Sources */,
				97C146EB1CF9000F007C117D /* Frameworks */,
				97C146EC1CF9000F007C117D /* Resources */,
				9705A1C41CF9048500538489 /* Embed Frameworks */,
				3B06AD1E1E4923F5004D2608 /* Thin Binary */,
			);
			buildRules = (
			);
			dependencies = (
			);
			name = Runner;`,
`			buildPhases = (
				9740EEB61CF901F6004384FC /* Run Script */,
				97C146EA1CF9000F007C117D /* Sources */,
				97C146EB1CF9000F007C117D /* Frameworks */,
				97C146EC1CF9000F007C117D /* Resources */,
				${E1} /* Embed Foundation Extensions */,
				9705A1C41CF9048500538489 /* Embed Frameworks */,
				3B06AD1E1E4923F5004D2608 /* Thin Binary */,
			);
			buildRules = (
			);
			dependencies = (
			);
			name = Runner;`);

// 7. Project targets list + TargetAttributes
p = p.replace(`			targets = (
				97C146ED1CF9000F007C117D /* Runner */,
				331C8080294A63A400263BE5 /* RunnerTests */,
			);`,
`			targets = (
				97C146ED1CF9000F007C117D /* Runner */,
				${T1} /* PacketTunnel */,
				331C8080294A63A400263BE5 /* RunnerTests */,
			);`);

p = p.replace(`				TargetAttributes = {
					331C8080294A63A400263BE5 = {`,
`				TargetAttributes = {
					${T1} = {
						CreatedOnToolsVersion = 14.0;
					};
					331C8080294A63A400263BE5 = {`);

// 8. Sources phase for extension
p = p.replace('/* End PBXSourcesBuildPhase section */',
`		${S1} /* Sources */ = {
			isa = PBXSourcesBuildPhase;
			buildActionMask = 2147483647;
			files = (
				${A} /* PacketTunnelProvider.swift in Sources */,
			);
			runOnlyForDeploymentPostprocessing = 0;
		};
/* End PBXSourcesBuildPhase section */`);

// 9. Build configurations for extension (base style copied from Runner's)
const extBase = `				ASSETCATALOG_COMPILER_APPICON_NAME = AppIcon;
				CLANG_ENABLE_MODULES = YES;
				CODE_SIGN_ENTITLEMENTS = PacketTunnel/PacketTunnel.entitlements;
				CODE_SIGN_STYLE = Automatic;
				CURRENT_PROJECT_VERSION = "$(FLUTTter_BUILD_NUMBER)";`;

const extConfigs = `		${C1} /* Debug */ = {
			isa = XCBuildConfiguration;
			buildSettings = {
				CLANG_ENABLE_MODULES = YES;
				CODE_SIGN_ENTITLEMENTS = PacketTunnel/PacketTunnel.entitlements;
				CODE_SIGN_STYLE = Automatic;
				CURRENT_PROJECT_VERSION = "$(FLUTTER_BUILD_NUMBER)";
				INFOPLIST_FILE = PacketTunnel/Info.plist;
				LD_RUNPATH_SEARCH_PATHS = (
					"$(inherited)",
					"@executable_path/Frameworks",
					"@executable_path/../../Frameworks",
				);
				LIBRARY_SEARCH_PATHS = (
					"$(PROJECT_DIR)/PacketTunnel/Libs",
				);
				MARKETING_VERSION = 1.0;
				OTHER_LDFLAGS = "-lclash";
				PRODUCT_BUNDLE_IDENTIFIER = com.follow.clash.flClash.PacketTunnel;
				PRODUCT_NAME = "$(TARGET_NAME)";
				SKIP_INSTALL = YES;
				SWIFT_OBJC_BRIDGING_HEADER = "PacketTunnel/PacketTunnel-Bridging-Header.h";
				SWIFT_OPTIMIZATION_LEVEL = "-Onone";
				SWIFT_VERSION = 5.0;
				TARGETED_DEVICE_FAMILY = "1,2";
				VERSIONING_SYSTEM = "apple-generic";
			};
			name = Debug;
		};
		${C2} /* Release */ = {
			isa = XCBuildConfiguration;
			buildSettings = {
				CLANG_ENABLE_MODULES = YES;
				CODE_SIGN_ENTITLEMENTS = PacketTunnel/PacketTunnel.entitlements;
				CODE_SIGN_STYLE = Automatic;
				CURRENT_PROJECT_VERSION = "$(FLUTTER_BUILD_NUMBER)";
				INFOPLIST_FILE = PacketTunnel/Info.plist;
				LD_RUNPATH_SEARCH_PATHS = (
					"$(inherited)",
					"@executable_path/Frameworks",
					"@executable_path/../../Frameworks",
				);
				LIBRARY_SEARCH_PATHS = (
					"$(PROJECT_DIR)/PacketTunnel/Libs",
				);
				MARKETING_VERSION = 1.0;
				OTHER_LDFLAGS = "-lclash";
				PRODUCT_BUNDLE_IDENTIFIER = com.follow.clash.flClash.PacketTunnel;
				PRODUCT_NAME = "$(TARGET_NAME)";
				SKIP_INSTALL = YES;
				SWIFT_OBJC_BRIDGING_HEADER = "PacketTunnel/PacketTunnel-Bridging-Header.h";
				SWIFT_VERSION = 5.0;
				TARGETED_DEVICE_FAMILY = "1,2";
				VERSIONING_SYSTEM = "apple-generic";
			};
			name = Release;
		};
		${C3} /* Profile */ = {
			isa = XCBuildConfiguration;
			buildSettings = {
				CLANG_ENABLE_MODULES = YES;
				CODE_SIGN_ENTITLEMENTS = PacketTunnel/PacketTunnel.entitlements;
				CODE_SIGN_STYLE = Automatic;
				CURRENT_PROJECT_VERSION = "$(FLUTTER_BUILD_NUMBER)";
				INFOPLIST_FILE = PacketTunnel/Info.plist;
				LD_RUNPATH_SEARCH_PATHS = (
					"$(inherited)",
					"@executable_path/Frameworks",
					"@executable_path/../../Frameworks",
				);
				LIBRARY_SEARCH_PATHS = (
					"$(PROJECT_DIR)/PacketTunnel/Libs",
				);
				MARKETING_VERSION = 1.0;
				OTHER_LDFLAGS = "-lclash";
				PRODUCT_BUNDLE_IDENTIFIER = com.follow.clash.flClash.PacketTunnel;
				PRODUCT_NAME = "$(TARGET_NAME)";
				SKIP_INSTALL = YES;
				SWIFT_OBJC_BRIDGING_HEADER = "PacketTunnel/PacketTunnel-Bridging-Header.h";
				SWIFT_VERSION = 5.0;
				TARGETED_DEVICE_FAMILY = "1,2";
				VERSIONING_SYSTEM = "apple-generic";
			};
			name = Profile;
		};
/* End XCBuildConfiguration section */`;
p = p.replace('/* End XCBuildConfiguration section */', extConfigs);

// 10. Config list for extension
p = p.replace('/* End XCConfigurationList section */',
`		${L1} /* Build configuration list for PBXNativeTarget "PacketTunnel" */ = {
			isa = XCConfigurationList;
			buildConfigurations = (
				${C1} /* Debug */,
				${C2} /* Release */,
				${C3} /* Profile */,
			);
			defaultConfigurationIsVisible = 0;
			defaultConfigurationName = Release;
		};
/* End XCConfigurationList section */`);

// 11. Runner: CODE_SIGN_ENTITLEMENTS in all three configs
p = p.replace(/INFOPLIST_FILE = Runner\/Info\.plist;/g,
`CODE_SIGN_ENTITLEMENTS = Runner/Runner.entitlements;
				INFOPLIST_FILE = Runner/Info.plist;`);

fs.writeFileSync(projectPath, p);
console.log('PacketTunnel target injected');
