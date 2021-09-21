# Change Log

All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/).

## [Unreleased]

## [0.3.1] - 2021-09-21

### Changed

- Update go to 1.17 ([#18](https://github.com/zoetrope/website-operator/pull/18))

## [0.3.0] - 2021-07-03

### Added

- Feature AfterBuildScript by using batchc1/Job ([#16](https://github.com/zoetrope/website-operator/pull/16))

### Changed

- Update controller-runtime to v0.9.2 ([#14](https://github.com/zoetrope/website-operator/pull/14))
- Fix RevisionWatcher is passing of incorrect address ([#15](https://github.com/zoetrope/website-operator/pull/15))

## [0.2.2] - 2021-03-04

### Added
- Add namespace field for buildScript and extraResource([#13](https://github.com/zoetrope/website-operator/pull/13))

## [0.2.1] - 2021-03-04

### Added
- Support imagePullSecrets ([#12](https://github.com/zoetrope/website-operator/pull/12))

## [0.2.0] - 2021-03-03

### Changed
- Stop access to secret resources ([#11](https://github.com/zoetrope/website-operator/pull/11))

## [0.1.2] - 2021-03-03

### Changed
- Support Kubernetes v1.20 ([#10](https://github.com/zoetrope/website-operator/pull/10))
- Fixed the bug that repo-checker will fail to clone the target repository ([#9](https://github.com/zoetrope/website-operator/pull/9))

## [0.1.1] - 2021-01-05

### Added

- Support secret resource for build script ([#6](https://github.com/zoetrope/website-operator/pull/6))

### Changed

- Fixed the problem that prevented detection of revision changes ([#7](https://github.com/zoetrope/website-operator/pull/7))

## [0.1.0] - 2020-11-13

### Added

- Add Web UI ([#5](https://github.com/zoetrope/website-operator/pull/5))

## [0.0.3] - 2020-11-10

### Added

- Apply podTemplate to repo-checker deployment (only labels and annotations) ([#4](https://github.com/zoetrope/website-operator/pull/4))

## [0.0.2] - 2020-11-04

### Added

- Support for replicas ([#1](https://github.com/zoetrope/website-operator/pull/1))
- Support for podTemplate and serviceTemplate (only labels and annotations) ([#3](https://github.com/zoetrope/website-operator/pull/3))

### Changed

- Change the name of website-operator ([#2](https://github.com/zoetrope/website-operator/pull/2))

## [0.0.1] - 2020-11-03

This is the first public release.

[Unreleased]: https://github.com/zoetrope/website-operator/compare/v0.3.1...HEAD
[0.3.0]: https://github.com/zoetrope/website-operator/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/zoetrope/website-operator/compare/v0.2.2...v0.3.0
[0.2.2]: https://github.com/zoetrope/website-operator/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/zoetrope/website-operator/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/zoetrope/website-operator/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/zoetrope/website-operator/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/zoetrope/website-operator/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/zoetrope/website-operator/compare/v0.0.3...v0.1.0
[0.0.3]: https://github.com/zoetrope/website-operator/compare/v0.0.2...v0.0.3
[0.0.2]: https://github.com/zoetrope/website-operator/compare/v0.0.1...v0.0.2
[0.0.1]: https://github.com/zoetrope/website-operator/compare/fd94306d63596e50c351fea50fba819c1aa349bc...v0.0.1
