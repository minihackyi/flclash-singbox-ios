// Adds Runner->PacketTunnel dependency and proper appex embed build file.
const fs = require('fs');
const path = require('path');
const projectPath = path.join(__dirname, '..', 'ios', 'Runner.xcodeproj', 'project.pbxproj');
let p = fs.readFileSync(projectPath, 'utf8');
if (p.includes('F200000000000000000000A')) {
  console.log('dependency already present, skipping');
  process.exit(0);
}
const AB = 'F200000000000000000000A'; // build file PacketTunnel.appex in Embed
const P1 = 'F100000000000000000000P1'; // product appex fileref
const E1 = 'F100000000000000000000E1'; // embed phase
const T1 = 'F100000000000000000000T1'; // PacketTunnel target
const X1 = 'F200000000000000000000X1'; // container proxy
const D1 = 'F200000000000000000000D1'; // target dependency

p = p.replace('/* Begin PBXBuildFile section */',
`/* Begin PBXBuildFile section */
		${AB} /* PacketTunnel.appex in Embed Foundation Extensions */ = {isa = PBXBuildFile; fileRef = ${P1} /* PacketTunnel.appex */; settings = {ATTRIBUTES = (RemoveHeadersOnEmbed, ); }; };`);

// fix embed phase files list: replace bare product ref with build file id
p = p.replace(`			dstSubfolderSpec = 13;
			files = (
				${P1} /* PacketTunnel.appex */,
			);`,
`			dstSubfolderSpec = 13;
			files = (
				${AB} /* PacketTunnel.appex in Embed Foundation Extensions */,
			);`);

// container item proxy + target dependency sections
p = p.replace('/* End PBXContainerItemProxy section */',
`		${X1} /* PBXContainerItemProxy */ = {
			isa = PBXContainerItemProxy;
			containerPortal = 97C146E61CF9000F007C117D /* Project object */;
			proxyType = 1;
			remoteGlobalIDString = ${T1};
			remoteInfo = PacketTunnel;
		};
/* End PBXContainerItemProxy section */`);

p = p.replace('/* End PBXTargetDependency section */',
`		${D1} /* PBXTargetDependency */ = {
			isa = PBXTargetDependency;
			target = ${T1} /* PacketTunnel */;
			targetProxy = ${X1} /* PBXContainerItemProxy */;
		};
/* End PBXTargetDependency section */`);

// Runner dependencies
p = p.replace(`			productType = "com.apple.product-type.application";
		};
		F100000000000000000000T1`,
`			productType = "com.apple.product-type.application";
		};`);
// insert dependency into Runner's native target block
p = p.replace(`			buildRules = (
			);
			dependencies = (
			);
			name = Runner;`,
`			buildRules = (
			);
			dependencies = (
				${D1} /* PBXTargetDependency */,
			);
			name = Runner;`);

fs.writeFileSync(projectPath, p);
console.log('dependency + embed fixed');
