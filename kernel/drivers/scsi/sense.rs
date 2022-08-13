// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Constants and parsing helper functionality for
// SCSI sense data.

/// Parse a set of sense data.
///
pub fn parse_sense_data(sense: &[u8]) -> (Option<SenseKey>, Option<AdditionalSense>) {
    // For now, we only support the fixed size
    // sense format.
    if sense.len() < 14 {
        return (None, None);
    }

    let response_code = sense[0] & 0b0111_1111;
    if response_code != 0x70 && response_code != 0x71 {
        return (None, None);
    }

    // See https://www.seagate.com/files/staticfiles/support/docs/manual/Interface%20manuals/100293068j.pdf,
    // table 29, pages 60-64.
    let sense_key = sense[2] & 0xf;
    let additional_sense_code = sense[12];
    let additional_sense_qualifier = sense[13];

    (
        Some(SenseKey::from_u8(sense_key)),
        AdditionalSense::from_sense_data(
            sense_key,
            additional_sense_code,
            additional_sense_qualifier,
        ),
    )
}

/// A SCSI sense key.
///
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub enum SenseKey {
    NoSense = 0x0,
    RecoveredError = 0x1,
    NotReady = 0x2,
    MediumError = 0x3,
    HardwareError = 0x4,
    IllegalRequest = 0x5,
    UnitAttention = 0x6,
    DataProtect = 0x7,
    BlankCheck = 0x8,
    VendorSpecific = 0x9,
    CopyAborted = 0xa,
    AbortedCommand = 0xb,
    Reserved = 0xc,
    VolumeOverflow = 0xd,
    Miscompare = 0xe,
    Completed = 0xf,
}

impl SenseKey {
    #[allow(clippy::missing_panics_doc)]
    pub fn from_u8(key: u8) -> Self {
        match key & 0xf {
            0x0 => SenseKey::NoSense,
            0x1 => SenseKey::RecoveredError,
            0x2 => SenseKey::NotReady,
            0x3 => SenseKey::MediumError,
            0x4 => SenseKey::HardwareError,
            0x5 => SenseKey::IllegalRequest,
            0x6 => SenseKey::UnitAttention,
            0x7 => SenseKey::DataProtect,
            0x8 => SenseKey::BlankCheck,
            0x9 => SenseKey::VendorSpecific,
            0xa => SenseKey::CopyAborted,
            0xb => SenseKey::AbortedCommand,
            0xc => SenseKey::Reserved,
            0xd => SenseKey::VolumeOverflow,
            0xe => SenseKey::Miscompare,
            0xf => SenseKey::Completed,
            #[allow(clippy::panic)]
            _ => panic!("unreachable"),
        }
    }
}

/// Additional sense codes and qualifiers.
///
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub enum AdditionalSense {
    NoSense,
    NoIndexOrLogicalBlockSignal,
    NoSeekComplete,
    PeripheralDeviceWriteFault,
    WriteFaultDataCorruption,
    LogicalUnitNotReadyAndCauseNotReportable,
    LogicalUnitNotReadyButBecomingReady,
    LogicalUnitNotReadyAndStartUnitRequired,
    LogicalUnitNotReadyAndManualInterventionRequired,
    LogicalUnitNotReadyAndFormatInProgress,
    LogicalUnitNotReadyAndSelfTestInProgress,
    LogicalUnitNotReadyAndNvcRecoveryInProgressAfterAnExceptionEvent,
    LogicalUnitNotReadyAndNotifyRequired,
    LogicalUnitNotReadyAndPowerCycleRequired,
    LogicalUnitNotReadyAndSuperCertifyInProgress,
    LogicalUnitCommunicationFailure,
    LogicalUnitCommunicationTimeOut,
    LogicalUnitCommunicationParityError,
    TrackFollowingError,
    ServoFault,
    HeadSelectFault,
    WriteToAtLeastOneCopyOfARedundantFileFailed,
    RedundantFilesHaveBelowFiftyPercentGoodCopies,
    CalibrationIsNeededButTheQstIsSetWithoutTheRecalOnlyBit,
    ServoCalCompletedAsPartOfSelfTest,
    ErrorLogOverflow,
    FailedToWriteSuperCertifyLogFile,
    FailedToReadSuperCertifyLogFile,
    AbortedCommand,
    WarningSpecifiedTemperatureExceeded,
    WarningEnclosureDegraded,
    WriteError,
    WriteErrorRecoveredWithAutoReallocation,
    WriteErrorAndAutoReallocationFailed,
    WriteErrorAndRecommendedReassignment,
    WriteErrorAndTooManyErrorRecoveryRevs,
    VolumeOverflowConstants,
    DataMiscompare,
    IdCrcOrEccError,
    UnrecoveredReadError,
    ReadRetriesExhausted,
    ErrorTooLongToCorrect,
    UnrecoveredReadErrorAndAutoReallocationFailed,
    UnrecoveredReadErrorAndTooManyErrorRecoveryRevs,
    AddressMarkNotFoundForIdField,
    RecoveredDataWithoutEccUsingPreviousLogicalBlockId,
    RecoveredDataWithEccUsingPreviousLogicalBlockId,
    LogicalBlockNotFound,
    RecordNotFound,
    RandomPositioningError,
    MechanicalPositioningError,
    PositioningErrorDetectedByReadOfMedium,
    DataSynchronizationMarkError,
    RecoveredDataWithNoErrorCorrectionApplied,
    RecoveredDataUsingRetries,
    RecoveredDataUsingPositiveOffset,
    RecoveredDataUsingNegativeOffset,
    RecoveredDataUsingPreviousLogicalBlockId,
    RecoveredDataWithoutEccAndDataAutoReallocated,
    RecoveredDataWithEcc,
    RecoveredDataWithEccAndRetriesApplied,
    RecoveredDataWithEccAndOrRetriesAndDataAutoReallocated,
    RecoveredDataAndRecommendReassignment,
    RecoveredDataUsingEccAndOffsets,
    RecoveredDataWithEccAndDataRewritten,
    DefectListError,
    DefectListNotAvailable,
    DefectListErrorInPrimaryList,
    DefectListErrorInGrownList,
    FewerThanFiftyPercentDefectListCopies,
    ParameterListLengthError,
    SynchronousDataTransferError,
    DefectListNotFound,
    PrimaryDefectListNotFound,
    GrownDefectListNotFound,
    SeagateUniqueDiagnosticCode,
    MiscompareDuringVerifyOperation,
    NumberOfDefectsOverflowsTheAllocatedSpaceThatTheReadDefectCommandCanHandle,
    InvalidCommandOperationCode,
    InvalidLinkedCommandOperationCode,
    LogicalBlockAddressOutOfRange,
    InvalidFieldInCdb,
    IllegalQueueTypeForCdb,
    InvalidLbaInLinkedCommand,
    IllegalGToPOperationRequest,
    LogicalUnitNotSupported,
    InvalidFieldInParameterList,
    ParameterNotSupported,
    ParameterValueInvalid,
    InvalidFieldParameterForThresholdParameter,
    InvalidReleaseOfActivePersistentReserve,
    FailToReadValidLogDumpData,
    InvalidFieldParameterForTmsFirmwareTag,
    InvalidFieldParameterForCheckSum,
    InvalidFieldParameterForFirmwareTag,
    WriteProtected,
    FlashingLedOccurred,
    PowerOnResetOrBusDeviceResetOccurred,
    PowerOnResetOccurred,
    ScsiBusResetOccurred,
    BusDeviceResetFunctionOccurred,
    InternalResetOccurred,
    TransceiverModeChangedToSingleEnded,
    TransceiverModeChangedToLvd,
    WriteLogDumpDataToDiskSuccessfulOrItNexusLoss,
    WriteLogDumpDataToDiskFail,
    WriteLogDumpEntryInformationFail,
    ReservedDiskSpaceIsFull,
    SdbpTestServiceContainedAnErrorAndExamineStatusPacketsForDetails,
    SdbpIncomingBufferOverflow,
    FlashingLedOccurredAndColdReset,
    FlashingLedOccurredAndWarmReset,
    ModeParametersChanged,
    LogParametersChanged,
    ReservationsPreempted,
    ReservationsReleased,
    RegistrationsPreempted,
    CommandSequenceError,
    TaggedCommandsClearedByAnotherInitiator,
    MediumFormatCorrupted,
    CorruptionInRwFormatRequest,
    CorruptWorldWideNameInDriveInformationFile,
    NoDefectSpareLocationAvailable,
    DefectListUpdateError,
    NoSparesAvailableAndTooManyDefectsOnOneTrack,
    DefectListLongerThanAllocatedMemory,
    FlashNotReadyForAccess,
    UnspecifiedEnclosureServicesFailure,
    UnsupportedEnclosureFunction,
    EnclosureServicesUnavailable,
    EnclosureTransferFailure,
    EnclosureTransferRefused,
    ParameterRounded,
    InvalidBitsInIdentityMessage,
    LogicalUnitFailedSelfTest,
    LogicalUnitHasNotSelfConfiguredYet,
    TargetOperatingConditionsHaveChanged,
    DeviceInternalResetOccurred,
    ChangedOperatingDefinition,
    DeviceIdentifierChanged,
    EchoBufferOverwritten,
    BufferContentsHaveChanged,
    InvalidApmParameters,
    WorldWideNameMismatch,
    DramParityError,
    SpinupErrorRecoveredWithRetries,
    PowerOnOrSelfTestFailure,
    PortAFailedLoopbackTest,
    PortBFailedLoopbackTest,
    MessageRejectError,
    InternalTargetFailure,
    DataIntegrityCheckFailedOnVerify,
    DataIntegrityCheckFailedDuringWrite,
    XorCdbCheckError,
    SelectReselectionFailure,
    ScsiParityError,
    InformationUnitCrcError,
    FibreChannelSequenceError,
    InitiatorDetectedErrorMessageReceived,
    InvalidMessageReceived,
    DataPhaseError,
    InvalidTransferTag,
    TooManyWriteData,
    AckNakTimeout,
    NakReceived,
    DataOffsetError,
    InitiatorResponseTimeout,
    LogicalUnitFailedSelfConfiguration,
    OverlappedCommandsAttempted,
    XorCacheIsNotAvailable,
    PrktTableIsFull,
    LogException,
    ThresholdConditionMet,
    LogCounterAtMaximum,
    LogListCodesExhausted,
    RplStatusChange,
    SpindlesSynchronized,
    SpindlesNotSynchronized,
    FailurePredictionThresholdExceeded,
    FalseFailurePredictionThresholdExceeded,
    VoltageFault,
    GeneralFirmwareErrorQualifier,
    IoedcErrorOnRead,
    IoedcErrorOnWrite,
    HostParityCheckFailed,
    IoedcErrorOnReadDetectedByFormatter,
    HostFifoParityErrorDetectedByCommonBuffer,
    HostFifoParityErrorDetectedByFrameBufferLogic,
    HostDataFrameBufferParityError,
    ReassignPowerAndFailRecoveryFailed,
    LaCheckErrorAndLcmBitIsZero,
    LaCheckError,
    UnreportedDeferredErrorsHaveBeenLoggedOnLogPage0x34,
}

impl AdditionalSense {
    pub fn from_sense_data(
        additional_sense_code: u8,
        additional_sense_qualifier: u8,
        sense_key: u8,
    ) -> Option<Self> {
        match (additional_sense_code, additional_sense_qualifier, sense_key) {
            (0x00, 0x00, 0x0) => Some(AdditionalSense::NoSense),
            (0x01, 0x00, 0x4) => Some(AdditionalSense::NoIndexOrLogicalBlockSignal),
            (0x02, 0x00, 0x4) => Some(AdditionalSense::NoSeekComplete),
            (0x03, 0x00, 0x1) | (0x03, 0x00, 0x3) | (0x03, 0x00, 0x4) => Some(AdditionalSense::PeripheralDeviceWriteFault),
            (0x03, 0x86, _) => Some(AdditionalSense::WriteFaultDataCorruption),
            (0x04, 0x00, 0x2) => Some(AdditionalSense::LogicalUnitNotReadyAndCauseNotReportable),
            (0x04, 0x01, 0x2) => Some(AdditionalSense::LogicalUnitNotReadyButBecomingReady),
            (0x04, 0x02, 0x2) => Some(AdditionalSense::LogicalUnitNotReadyAndStartUnitRequired),
            (0x04, 0x03, 0x2) => Some(AdditionalSense::LogicalUnitNotReadyAndManualInterventionRequired),
            (0x04, 0x04, 0x2) => Some(AdditionalSense::LogicalUnitNotReadyAndFormatInProgress),
            (0x04, 0x09, 0x2) => Some(AdditionalSense::LogicalUnitNotReadyAndSelfTestInProgress),
            (0x04, 0x0a, 0x2) => Some(AdditionalSense::LogicalUnitNotReadyAndNvcRecoveryInProgressAfterAnExceptionEvent),
            (0x04, 0x11, 0x2) => Some(AdditionalSense::LogicalUnitNotReadyAndNotifyRequired),
            (0x04, 0x22, 0x2) => Some(AdditionalSense::LogicalUnitNotReadyAndPowerCycleRequired),
            (0x04, 0xf0, 0x2) => Some(AdditionalSense::LogicalUnitNotReadyAndSuperCertifyInProgress),
            (0x08, 0x00, 0x9) | (0x08, 0x00, 0xb) => Some(AdditionalSense::LogicalUnitCommunicationFailure),
            (0x08, 0x01, 0xb) => Some(AdditionalSense::LogicalUnitCommunicationTimeOut),
            (0x08, 0x02, _) => Some(AdditionalSense::LogicalUnitCommunicationParityError),
            (0x09, 0x00, 0x1) | (0x09, 0x00, 0x3) | (0x09, 0x00, 0x4) => Some(AdditionalSense::TrackFollowingError),
            (0x09, 0x01, 0x1) | (0x09, 0x01, 0x4) => Some(AdditionalSense::ServoFault),
            (0x09, 0x04, 0x3) | (0x09, 0x04, 0x4) => Some(AdditionalSense::HeadSelectFault),
            (0x09, 0x0d, 0x1) => Some(AdditionalSense::WriteToAtLeastOneCopyOfARedundantFileFailed),
            (0x09, 0x0e, 0x1) => Some(AdditionalSense::RedundantFilesHaveBelowFiftyPercentGoodCopies),
            (0x09, 0xf8, 0x1) => Some(AdditionalSense::CalibrationIsNeededButTheQstIsSetWithoutTheRecalOnlyBit),
            (0x09, 0xff, 0x1) => Some(AdditionalSense::ServoCalCompletedAsPartOfSelfTest),
            (0x0a, 0x00, _) => Some(AdditionalSense::ErrorLogOverflow),
            (0x0a, 0x01, 0x3) => Some(AdditionalSense::FailedToWriteSuperCertifyLogFile),
            (0x0a, 0x02, 0x3) => Some(AdditionalSense::FailedToReadSuperCertifyLogFile),
            (0x0b, 0x00, 0xb) => Some(AdditionalSense::AbortedCommand),
            (0x0b, 0x01, 0x1) | (0x0b, 0x01, 0x6) => Some(AdditionalSense::WarningSpecifiedTemperatureExceeded),
            (0x0b, 0x02, 0x1) => Some(AdditionalSense::WarningEnclosureDegraded),
            (0x0c, 0x00, 0x3) => Some(AdditionalSense::WriteError),
            (0x0c, 0x01, 0x1) => Some(AdditionalSense::WriteErrorRecoveredWithAutoReallocation),
            (0x0c, 0x02, 0x3) => Some(AdditionalSense::WriteErrorAndAutoReallocationFailed),
            (0x0c, 0x03, 0x3) => Some(AdditionalSense::WriteErrorAndRecommendedReassignment),
            (0x0c, 0xff, 0x3) => Some(AdditionalSense::WriteErrorAndTooManyErrorRecoveryRevs),
            (0x0d, 0x00, 0xd) => Some(AdditionalSense::VolumeOverflowConstants),
            (0x0e, 0x00, 0xe) => Some(AdditionalSense::DataMiscompare),
            (0x10, 0x00, _) => Some(AdditionalSense::IdCrcOrEccError),
            (0x11, 0x00, 0x1) | (0x11, 0x00, 0x3) => Some(AdditionalSense::UnrecoveredReadError),
            (0x11, 0x01, _) => Some(AdditionalSense::ReadRetriesExhausted),
            (0x11, 0x02, _) => Some(AdditionalSense::ErrorTooLongToCorrect),
            (0x11, 0x04, 0x3) => Some(AdditionalSense::UnrecoveredReadErrorAndAutoReallocationFailed),
            (0x11, 0xff, 0x3) => Some(AdditionalSense::UnrecoveredReadErrorAndTooManyErrorRecoveryRevs),
            (0x12, 0x00, _) => Some(AdditionalSense::AddressMarkNotFoundForIdField),
            (0x12, 0x01, _) => Some(AdditionalSense::RecoveredDataWithoutEccUsingPreviousLogicalBlockId),
            (0x12, 0x02, _) => Some(AdditionalSense::RecoveredDataWithEccUsingPreviousLogicalBlockId),
            (0x14, 0x00, _) => Some(AdditionalSense::LogicalBlockNotFound),
            (0x14, 0x01, 0x3) => Some(AdditionalSense::RecordNotFound),
            (0x15, 0x00, _) => Some(AdditionalSense::RandomPositioningError),
            (0x15, 0x01, 0x1) | (0x15, 0x01, 0x3) | (0x15, 0x01, 0x4) => Some(AdditionalSense::MechanicalPositioningError),
            (0x15, 0x02, _) => Some(AdditionalSense::PositioningErrorDetectedByReadOfMedium),
            (0x16, 0x00, 0x1) | (0x16, 0x00, 0x3) | (0x16, 0x00, 0x4) => Some(AdditionalSense::DataSynchronizationMarkError),
            (0x17, 0x00, _) => Some(AdditionalSense::RecoveredDataWithNoErrorCorrectionApplied),
            (0x17, 0x01, 0x1) => Some(AdditionalSense::RecoveredDataUsingRetries),
            (0x17, 0x02, 0x1) => Some(AdditionalSense::RecoveredDataUsingPositiveOffset),
            (0x17, 0x03, 0x1) => Some(AdditionalSense::RecoveredDataUsingNegativeOffset),
            (0x17, 0x05, _) => Some(AdditionalSense::RecoveredDataUsingPreviousLogicalBlockId),
            (0x17, 0x06, _) => Some(AdditionalSense::RecoveredDataWithoutEccAndDataAutoReallocated),
            (0x18, 0x00, 0x1) => Some(AdditionalSense::RecoveredDataWithEcc),
            (0x18, 0x01, 0x1) => Some(AdditionalSense::RecoveredDataWithEccAndRetriesApplied),
            (0x18, 0x02, 0x1) => Some(AdditionalSense::RecoveredDataWithEccAndOrRetriesAndDataAutoReallocated),
            (0x18, 0x05, _) => Some(AdditionalSense::RecoveredDataAndRecommendReassignment),
            (0x18, 0x06, _) => Some(AdditionalSense::RecoveredDataUsingEccAndOffsets),
            (0x18, 0x07, 0x1) => Some(AdditionalSense::RecoveredDataWithEccAndDataRewritten),
            (0x19, 0x00, 0x1) | (0x19, 0x00, 0x4) => Some(AdditionalSense::DefectListError),
            (0x19, 0x01, _) => Some(AdditionalSense::DefectListNotAvailable),
            (0x19, 0x02, _) => Some(AdditionalSense::DefectListErrorInPrimaryList),
            (0x19, 0x03, _) => Some(AdditionalSense::DefectListErrorInGrownList),
            (0x19, 0x0e, _) => Some(AdditionalSense::FewerThanFiftyPercentDefectListCopies),
            (0x1a, 0x00, 0x5) => Some(AdditionalSense::ParameterListLengthError),
            (0x1b, 0x00, _) => Some(AdditionalSense::SynchronousDataTransferError),
            (0x1c, 0x00, 0x1) | (0x1c, 0x00, 0x4) => Some(AdditionalSense::DefectListNotFound),
            (0x1c, 0x01, _) => Some(AdditionalSense::PrimaryDefectListNotFound),
            (0x1c, 0x02, _) => Some(AdditionalSense::GrownDefectListNotFound),
            (0x1c, 0x83, _) => Some(AdditionalSense::SeagateUniqueDiagnosticCode),
            (0x1d, 0x00, 0xe) => Some(AdditionalSense::MiscompareDuringVerifyOperation),
            (0x1f, 0x00, 0x1) => Some(AdditionalSense::NumberOfDefectsOverflowsTheAllocatedSpaceThatTheReadDefectCommandCanHandle),
            (0x20, 0x00, 0x5) => Some(AdditionalSense::InvalidCommandOperationCode),
            (0x20, 0xf3, 0x5) => Some(AdditionalSense::InvalidLinkedCommandOperationCode),
            (0x21, 0x00, 0xd) => Some(AdditionalSense::LogicalBlockAddressOutOfRange),
            (0x24, 0x00, 0x5) => Some(AdditionalSense::InvalidFieldInCdb),
            (0x24, 0x05, 0x5) => Some(AdditionalSense::IllegalQueueTypeForCdb),
            (0x24, 0xf0, 0x5) => Some(AdditionalSense::InvalidLbaInLinkedCommand),
            (0x24, 0xf2, 0x5) => Some(AdditionalSense::InvalidLinkedCommandOperationCode),
            (0x24, 0xf3, 0x5) => Some(AdditionalSense::IllegalGToPOperationRequest),
            (0x25, 0x00, 0x5) => Some(AdditionalSense::LogicalUnitNotSupported),
            (0x26, 0x00, 0x5) => Some(AdditionalSense::InvalidFieldInParameterList),
            (0x26, 0x01, 0x5) => Some(AdditionalSense::ParameterNotSupported),
            (0x26, 0x02, 0x5) => Some(AdditionalSense::ParameterValueInvalid),
            (0x26, 0x03, 0x5) => Some(AdditionalSense::InvalidFieldParameterForThresholdParameter),
            (0x26, 0x04, 0x5) => Some(AdditionalSense::InvalidReleaseOfActivePersistentReserve),
            (0x26, 0x05, 0x5) => Some(AdditionalSense::FailToReadValidLogDumpData),
            (0x26, 0x97, _) => Some(AdditionalSense::InvalidFieldParameterForTmsFirmwareTag),
            (0x26, 0x98, _) => Some(AdditionalSense::InvalidFieldParameterForCheckSum),
            (0x26, 0x99, _) => Some(AdditionalSense::InvalidFieldParameterForFirmwareTag),
            (0x27, 0x00, 0x7) => Some(AdditionalSense::WriteProtected),
            (0x29, 0x00, 0x4) => Some(AdditionalSense::FlashingLedOccurred),
            (0x29, 0x00, 0x6) => Some(AdditionalSense::PowerOnResetOrBusDeviceResetOccurred),
            (0x29, 0x01, 0x6) => Some(AdditionalSense::PowerOnResetOccurred),
            (0x29, 0x02, 0x6) => Some(AdditionalSense::ScsiBusResetOccurred),
            (0x29, 0x03, 0x6) => Some(AdditionalSense::BusDeviceResetFunctionOccurred),
            (0x29, 0x04, 0x6) => Some(AdditionalSense::InternalResetOccurred),
            (0x29, 0x05, 0x6) => Some(AdditionalSense::TransceiverModeChangedToSingleEnded),
            (0x29, 0x06, 0x6) => Some(AdditionalSense::TransceiverModeChangedToLvd),
            (0x29, 0x07, 0x6) => Some(AdditionalSense::WriteLogDumpDataToDiskSuccessfulOrItNexusLoss),
            (0x29, 0x08, 0x6) => Some(AdditionalSense::WriteLogDumpDataToDiskFail),
            (0x29, 0x09, 0x6) => Some(AdditionalSense::WriteLogDumpEntryInformationFail),
            (0x29, 0x0a, 0x6) => Some(AdditionalSense::ReservedDiskSpaceIsFull),
            (0x29, 0x0b, 0x6) => Some(AdditionalSense::SdbpTestServiceContainedAnErrorAndExamineStatusPacketsForDetails),
            (0x29, 0x0c, 0x6) => Some(AdditionalSense::SdbpIncomingBufferOverflow),
            (0x29, 0xcd, 0x6) => Some(AdditionalSense::FlashingLedOccurredAndColdReset),
            (0x29, 0xce, 0x6) => Some(AdditionalSense::FlashingLedOccurredAndWarmReset),
            (0x2a, 0x01, 0x6) => Some(AdditionalSense::ModeParametersChanged),
            (0x2a, 0x02, 0x6) => Some(AdditionalSense::LogParametersChanged),
            (0x2a, 0x03, 0x6) => Some(AdditionalSense::ReservationsPreempted),
            (0x2a, 0x04, 0x6) => Some(AdditionalSense::ReservationsReleased),
            (0x2a, 0x05, 0x6) => Some(AdditionalSense::RegistrationsPreempted),
            (0x2c, 0x00, 0x5) => Some(AdditionalSense::CommandSequenceError),
            (0x2f, 0x00, 0x6) => Some(AdditionalSense::TaggedCommandsClearedByAnotherInitiator),
            (0x31, 0x00, 0x3) => Some(AdditionalSense::MediumFormatCorrupted),
            (0x31, 0x01, 0x3) => Some(AdditionalSense::CorruptionInRwFormatRequest),
            (0x31, 0x91, 0x3) => Some(AdditionalSense::CorruptWorldWideNameInDriveInformationFile),
            (0x32, 0x00, 0x4) => Some(AdditionalSense::NoDefectSpareLocationAvailable),
            (0x32, 0x01, 0x3) | (0x32, 0x01, 0x4) | (0x32, 0x01, 0x5) => Some(AdditionalSense::DefectListUpdateError),
            (0x32, 0x02, _) => Some(AdditionalSense::NoSparesAvailableAndTooManyDefectsOnOneTrack),
            (0x32, 0x03, 0x3) => Some(AdditionalSense::DefectListLongerThanAllocatedMemory),
            (0x33, 0x00, 0x3) => Some(AdditionalSense::FlashNotReadyForAccess),
            (0x35, 0x00, 0x4) => Some(AdditionalSense::UnspecifiedEnclosureServicesFailure),
            (0x35, 0x01, 0x5) => Some(AdditionalSense::UnsupportedEnclosureFunction),
            (0x35, 0x02, 0x2) => Some(AdditionalSense::EnclosureServicesUnavailable),
            (0x35, 0x03, 0x4) => Some(AdditionalSense::EnclosureTransferFailure),
            (0x35, 0x04, 0x4) => Some(AdditionalSense::EnclosureTransferRefused),
            (0x37, 0x00, 0x1) => Some(AdditionalSense::ParameterRounded),
            (0x3d, 0x00, _) => Some(AdditionalSense::InvalidBitsInIdentityMessage),
            (0x3e, 0x03, 0x4) => Some(AdditionalSense::LogicalUnitFailedSelfTest),
            (0x3e, 0x00, _) => Some(AdditionalSense::LogicalUnitHasNotSelfConfiguredYet),
            (0x3f, 0x00, 0x6) => Some(AdditionalSense::TargetOperatingConditionsHaveChanged),
            (0x3f, 0x01, 0x6) => Some(AdditionalSense::DeviceInternalResetOccurred),
            (0x3f, 0x02, 0x6) => Some(AdditionalSense::ChangedOperatingDefinition),
            (0x3f, 0x05, 0x6) => Some(AdditionalSense::DeviceIdentifierChanged),
            (0x3f, 0x0f, 0xb) => Some(AdditionalSense::EchoBufferOverwritten),
            (0x3f, 0x80, 0x1) => Some(AdditionalSense::BufferContentsHaveChanged),
            (0x3f, 0x90, _) => Some(AdditionalSense::InvalidApmParameters),
            (0x3f, 0x91, 0x6) => Some(AdditionalSense::WorldWideNameMismatch),
            (0x40, 0x01, 0x1) | (0x40, 0x01, 0x4) => Some(AdditionalSense::DramParityError),
            (0x40, 0x02, 0x1) => Some(AdditionalSense::SpinupErrorRecoveredWithRetries),
            (0x42, 0x00, 0x4) => Some(AdditionalSense::PowerOnOrSelfTestFailure),
            (0x42, 0x0a, 0x4) => Some(AdditionalSense::PortAFailedLoopbackTest),
            (0x42, 0x0b, 0x4) => Some(AdditionalSense::PortBFailedLoopbackTest),
            (0x43, 0x00, 0xb) => Some(AdditionalSense::MessageRejectError),
            (0x44, 0x00, 0x1) | (0x44, 0x00, 0x3) | (0x44, 0x00, 0x4) => Some(AdditionalSense::InternalTargetFailure),
            (0x44, 0xf2, 0x4) => Some(AdditionalSense::DataIntegrityCheckFailedOnVerify),
            (0x44, 0xf6, 0x4) => Some(AdditionalSense::DataIntegrityCheckFailedDuringWrite),
            (0x44, 0xff, 0x4) => Some(AdditionalSense::XorCdbCheckError),
            (0x45, 0x00, 0xb) => Some(AdditionalSense::SelectReselectionFailure),
            (0x47, 0x00, 0xb) => Some(AdditionalSense::ScsiParityError),
            (0x47, 0x03, 0xb) => Some(AdditionalSense::InformationUnitCrcError),
            (0x47, 0x80, 0xb) => Some(AdditionalSense::FibreChannelSequenceError),
            (0x48, 0x00, 0xb) => Some(AdditionalSense::InitiatorDetectedErrorMessageReceived),
            (0x49, 0x00, 0xb) => Some(AdditionalSense::InvalidMessageReceived),
            (0x4b, 0x00, 0xb) => Some(AdditionalSense::DataPhaseError),
            (0x4b, 0x01, 0xb) => Some(AdditionalSense::InvalidTransferTag),
            (0x4b, 0x02, 0xb) => Some(AdditionalSense::TooManyWriteData),
            (0x4b, 0x03, 0xb) => Some(AdditionalSense::AckNakTimeout),
            (0x4b, 0x04, 0xb) => Some(AdditionalSense::NakReceived),
            (0x4b, 0x05, 0xb) => Some(AdditionalSense::DataOffsetError),
            (0x4b, 0x06, 0xb) => Some(AdditionalSense::InitiatorResponseTimeout),
            (0x4c, 0x00, _) => Some(AdditionalSense::LogicalUnitFailedSelfConfiguration),
            (0x4e, 0x00, 0xb) => Some(AdditionalSense::OverlappedCommandsAttempted),
            (0x55, 0x01, _) => Some(AdditionalSense::XorCacheIsNotAvailable),
            (0x55, 0x04, 0x5) => Some(AdditionalSense::PrktTableIsFull),
            (0x5b, 0x00, _) => Some(AdditionalSense::LogException),
            (0x5b, 0x01, _) => Some(AdditionalSense::ThresholdConditionMet),
            (0x5b, 0x02, _) => Some(AdditionalSense::LogCounterAtMaximum),
            (0x5b, 0x03, _) => Some(AdditionalSense::LogListCodesExhausted),
            (0x5c, 0x00, 0x6) => Some(AdditionalSense::RplStatusChange),
            (0x5c, 0x01, _) => Some(AdditionalSense::SpindlesSynchronized),
            (0x5c, 0x02, _) => Some(AdditionalSense::SpindlesNotSynchronized),
            (0x5d, 0x00, 0x1) | (0x5d, 0x00, 0x6) => Some(AdditionalSense::FailurePredictionThresholdExceeded),
            (0x5d, 0xff, 0x1) | (0x5d, 0xff, 0x6) => Some(AdditionalSense::FalseFailurePredictionThresholdExceeded),
            (0x65, 0x00, 0x4) => Some(AdditionalSense::VoltageFault),
            (0x80, 0x00, 0x9) => Some(AdditionalSense::GeneralFirmwareErrorQualifier),
            (0x80, 0x86, 0x9) => Some(AdditionalSense::IoedcErrorOnRead),
            (0x80, 0x87, 0x9) => Some(AdditionalSense::IoedcErrorOnWrite),
            (0x80, 0x88, 0x9) => Some(AdditionalSense::HostParityCheckFailed),
            (0x80, 0x89, 0x9) => Some(AdditionalSense::IoedcErrorOnReadDetectedByFormatter),
            (0x80, 0x8a, 0x9) => Some(AdditionalSense::HostFifoParityErrorDetectedByCommonBuffer),
            (0x80, 0x8b, 0x9) => Some(AdditionalSense::HostFifoParityErrorDetectedByFrameBufferLogic),
            (0x80, 0x8c, 0x9) => Some(AdditionalSense::HostDataFrameBufferParityError),
            (0x81, 0x00, 0x4) => Some(AdditionalSense::LaCheckErrorAndLcmBitIsZero),
            (0x81, 0x00, 0xb) => Some(AdditionalSense::LaCheckError),
            (0x81, 0x00, _) => Some(AdditionalSense::ReassignPowerAndFailRecoveryFailed),
            (0xb4, 0x00, 0x6) => Some(AdditionalSense::UnreportedDeferredErrorsHaveBeenLoggedOnLogPage0x34),
            _ => None,
        }
    }
}
