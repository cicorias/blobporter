package main

import (
	"fmt"

	"github.com/Azure/blobporter/pipeline"
	"github.com/Azure/blobporter/sources"
	"github.com/Azure/blobporter/targets"
	"github.com/Azure/blobporter/transfer"
)

func newTransferPipelines(params *validatedParameters) ([]pipeline.SourcePipeline, pipeline.TargetPipeline, error) {

	fact := newPipelinesFactory(params)

	var sourcesp []pipeline.SourcePipeline
	var targetp pipeline.TargetPipeline
	var err error

	targetp, err = fact.newTargetPipeline()

	if err != nil {
		return nil, nil, err
	}

	sourcesp, err = fact.newSourcePipelines()

	if err != nil {
		return nil, nil, err
	}

	return sourcesp, targetp, nil

}

/*
const openFileLimitForLinux = 1024

//validates if the number of readers needs to be adjusted to accommodate max filehandle limits in Debian systems
func adjustReaders(numOfSources int) error {
	if runtime.GOOS != "linux" {
		return nil
	}

	if (numOfSources * numberOfHandlesPerFile) > openFileLimitForLinux {
		numberOfHandlesPerFile = openFileLimitForLinux / (numOfSources * numberOfHandlesPerFile)

		if numberOfHandlesPerFile == 0 {
			return fmt.Errorf("The number of files will cause the process to exceed the limit of open files allowed by the OS. Reduce the number of files to be transferred")
		}

		fmt.Printf("Warning! Adjusting the number of handles per file (-h) to %v\n", numberOfHandlesPerFile)

	}

	return nil
}
*/
type pipelinesFactory struct {
	source    transfer.TransferSegment
	target    transfer.TransferSegment
	def       transfer.Definition
	valParams *validatedParameters
}
type parseAndValidationRule func() error

func newPipelinesFactory(params *validatedParameters) pipelinesFactory {

	s, t := transfer.ParseTransferSegment(params.transferType)
	p := pipelinesFactory{source: s, target: t, def: params.transferType, valParams: params}

	return p
}

func (p *pipelinesFactory) newTargetPipeline() (pipeline.TargetPipeline, error) {

	params, err := p.newTargetParams()

	if err != nil {
		return nil, err
	}

	switch p.target {
	case transfer.File:
		params := params.(targets.FileTargetParams)
		return targets.NewMultiFile(params.Overwrite, params.NumberOfHandlesPerFile), nil
	case transfer.BlockBlob:
		return targets.NewAzureBlockPipeline(params.(targets.AzureTargetParams)), nil
	case transfer.PageBlob:
		return targets.NewAzurePagePipeline(params.(targets.AzureTargetParams)), nil
	case transfer.Perf:
		return targets.NewPerfTargetPipeline(), nil
	}

	return nil, fmt.Errorf("Invalid target segment:%v", p.target)
}

func (p *pipelinesFactory) newSourcePipelines() ([]pipeline.SourcePipeline, error) {

	params, err := p.newSourceParams()

	if err != nil {
		return nil, err
	}

	switch p.source {
	case transfer.File:
		params := params.(sources.MultiFileParams)
		return sources.NewMultiFile(&params), nil
	case transfer.HTTP:
		params := params.(sources.HTTPSourceParams)
		return []pipeline.SourcePipeline{sources.NewHTTP(params.SourceURIs, params.TargetAliases, params.SourceParams.CalculateMD5)}, nil
	case transfer.S3:
		params := params.(sources.S3Params)
		return sources.NewS3Pipeline(&params), nil
	case transfer.Blob:
		params := params.(sources.AzureBlobParams)
		return sources.NewAzureBlob(&params), nil
	case transfer.Perf:
		return sources.NewPerfSourcePipeline(params.(sources.PerfSourceParams)), nil
	}

	return nil, fmt.Errorf("Invalid source segment:%v", p.source)
}
func (p *pipelinesFactory) newSourceParams() (interface{}, error) {

	switch p.source {
	case transfer.File:
		return sources.MultiFileParams{
			SourcePatterns:   p.valParams.sourceURIs,
			BlockSize:        p.valParams.blockSize,
			TargetAliases:    p.valParams.targetAliases,
			NumOfPartitions:  p.valParams.numberOfReaders, //TODO make this more explicit by numofpartitions as param..
			MD5:              p.valParams.calculateMD5,
			KeepDirStructure: p.valParams.keepDirStructure,
			FilesPerPipeline: p.valParams.numberOfIlesInBatch}, nil
	case transfer.HTTP:
		return sources.HTTPSourceParams{
			SourceURIs:    p.valParams.sourceURIs,
			TargetAliases: p.valParams.targetAliases,
			SourceParams: sources.SourceParams{
				CalculateMD5: p.valParams.calculateMD5}}, nil
	case transfer.S3:
		return sources.S3Params{
			Bucket:          p.valParams.s3Source.bucket,
			Prefixes:        p.valParams.s3Source.prefixes,
			Endpoint:        p.valParams.s3Source.endpoint,
			PreSignedExpMin: p.valParams.s3Source.preSignedExpMin,
			AccessKey:       p.valParams.s3Source.accessKey,
			SecretKey:       p.valParams.s3Source.secretKey,
			SourceParams: sources.SourceParams{
				CalculateMD5:      p.valParams.calculateMD5,
				UseExactNameMatch: p.valParams.useExactMatch,
				FilesPerPipeline:  p.valParams.numberOfIlesInBatch,
				//default to always true so blob names are kept
				KeepDirStructure: p.valParams.keepDirStructure}}, nil
	case transfer.Blob:
		return sources.AzureBlobParams{
			Container:   p.valParams.blobSource.container,
			BlobNames:   p.valParams.blobSource.prefixes,
			AccountName: p.valParams.blobSource.accountName,
			AccountKey:  p.valParams.blobSource.accountKey,
			BaseBlobURL: p.valParams.blobSource.baseBlobURL,
			SourceParams: sources.SourceParams{
				CalculateMD5:      p.valParams.calculateMD5,
				UseExactNameMatch: p.valParams.useExactMatch,
				FilesPerPipeline:  p.valParams.numberOfIlesInBatch,
				KeepDirStructure:  p.valParams.keepDirStructure}}, nil
	case transfer.Perf:
		return sources.PerfSourceParams{
			Definitions: p.valParams.perfSourceDefinitions,
			SourceParams: sources.SourceParams{
				CalculateMD5: p.valParams.calculateMD5}}, nil
	}

	return nil, fmt.Errorf("Invalid segment type: %v ", p.source)

}

func (p *pipelinesFactory) newTargetParams() (interface{}, error) {

	switch p.target {
	case transfer.File:
		return targets.FileTargetParams{
			Overwrite:              true, //set this to always overwrite, TODO, expose this as an option
			NumberOfHandlesPerFile: p.valParams.numberOfHandlesPerFile}, nil
	case transfer.PageBlob:
		return targets.AzureTargetParams{
			AccountName: p.valParams.blobTarget.accountName,
			AccountKey:  p.valParams.blobTarget.accountKey,
			Container:   p.valParams.blobTarget.container,
			BaseBlobURL: p.valParams.blobTarget.baseBlobURL}, nil
	case transfer.BlockBlob:
		return targets.AzureTargetParams{
			AccountName: p.valParams.blobTarget.accountName,
			AccountKey:  p.valParams.blobTarget.accountKey,
			Container:   p.valParams.blobTarget.container,
			BaseBlobURL: p.valParams.blobTarget.baseBlobURL}, nil
	case transfer.Perf:
		return nil, nil
	}

	return nil, fmt.Errorf("Invalid target segment type: %v ", p.target)
}
