package oscutil

import (
	"encoding/json"
	"github.com/E-Health/goscar"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"os"
	"strconv"
	"github.com/google/uuid"
    "regexp"
	"time"
)

// FhirObservation : Extended Observation as the existing one does not have valueString and valueInteger
type FhirObservation struct {
	Id           string            `json:"id,omitempty"`
	Identifier   []fhir.Identifier `json:"identifier,omitempty"`
	Subject      fhir.Reference    `json:"subject,omitempty"`
	ValueString  string            `json:"valueString,omitempty"`
	ValueInteger int               `json:"valueInteger,omitempty"`
	ResourceType string            `json:"resourceType"`
	Status       fhir.ObservationStatus `json:"status"`
	Code         fhir.CodeableConcept             `json:"code"`
}

// MapToFHIR : maps the csvMap to a FHIR bundle.
func MapToFHIR(_csvMapValid []map[string]string) fhir.Bundle {
	var composition = fhir.Composition{}
	var patient = fhir.Patient{}
	var observation = FhirObservation{} // Extended observation: See above for definition
	var identifier = fhir.Identifier{}
	var bundle = fhir.Bundle{}
	var reference = fhir.Reference{}
	var bundleEntry = fhir.BundleEntry{}
	var codableConcept = fhir.CodeableConcept{}
	var bundleType = fhir.BundleType(fhir.BundleTypeDocument) // The bundle is a document. The first resource is a Composition.
	var practitioner = fhir.Practitioner{}
	id := uuid.New()
	patients := []string{}

	location := os.Getenv("GOSCAR_LOCATION")
	username := os.Getenv("USER_NAME")
	mySystem := os.Getenv("GOSCAR_SYSTEM")
	mySeparator := os.Getenv("GOSCAR_ID_SEPARATOR")
	myUrn := os.Getenv("GOSCAR_URN")

	toIgnore := []string{"id", "fdid", "dateCreated", "eform_link", "StaffSig", "SubmitButton", "efmfid"}

	// Create a practitioner who is the author and the subject of composition
	practitionerId := location + mySeparator + username
	practitionerRefId := "Practitioner/" + practitionerId 
	practitioner.Id = &practitionerId

	// Single composition
	myTitle := location + mySeparator + username
	identifier.System = &mySystem
	identifier.Value = &myTitle
	composition.Identifier = &identifier
	composition.Status = fhir.CompositionStatus(fhir.CompositionStatusFinal) // Required
	// Random UUID for composition
	compositionId := id.String()
	composition.Id = &compositionId
	composition.Title = myTitle
	dt := time.Now()
	composition.Date = dt.Format("2006-01-02")
	// Set author as author and subject
	reference.Reference = &practitionerRefId
	composition.Author = append(composition.Author, reference)
	composition.Subject = &reference
	codableText := myUrn + "E-Form"
	codableConcept.Text = &codableText
	composition.Type = codableConcept


	// Single bundle
	identifier.System = &mySystem
	identifier.Value = &location
	bundle.Identifier = &identifier
	bundle.Type = bundleType // The bundle is a document. The first resource is a Composition.
	//bundleEntry.Id = &mySystem
	bundleTimestamp := dt.UTC().Format("2006-01-02T15:04:05Z")
	bundle.Timestamp = &bundleTimestamp

	// Add composition
	bundleEntry.Resource, _ = composition.MarshalJSON()
	myCompositionEntry := myUrn + compositionId
	bundleEntry.FullUrl = &myCompositionEntry
	bundle.Entry = append(bundle.Entry, bundleEntry)

	// Add practitioner
	bundleEntry.Resource, _ = practitioner.MarshalJSON()
	myPractitionerEntry := myUrn + practitionerId
	bundleEntry.FullUrl = &myPractitionerEntry
	bundle.Entry = append(bundle.Entry, bundleEntry)
	
	// Get headers from the first row
	headers := _csvMapValid[0]
	for _, record := range _csvMapValid {

		// Each record has a patient (ID is unique for location)
		patientId := location + mySeparator + record["demographicNo"]
		refPatientId := "Patient/" + patientId
		identifier.System = &mySystem
		identifier.Value = &myTitle
		_identifier := []fhir.Identifier{}
		_identifier = append(_identifier, identifier)
		patient.Identifier = _identifier
		patient.Id = &patientId // Needed for Reference

		// Each value is an observation
		for header, myval := range headers {
			// Function call to get the type of header -> number or string
			headerStat := goscar.GetStats(header, RecordCount, _csvMapValid)
			identifier.System = &mySystem
			identifier.Value = &header
			_identifier := []fhir.Identifier{}
			_identifier = append(_identifier, identifier)
			observation.Identifier = _identifier
			// Create a unique ID for observation to be added to key to generate the final ID
			observationId := location + mySeparator +
				record["efmfid"] + mySeparator +
				record["fdid"] + mySeparator +
				record["dateCreated"] + mySeparator + header
			reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
			observationId = reg.ReplaceAllString(observationId, "")
			observation.Id = observationId
			if !goscar.IsMember(header, toIgnore) {
				if headerStat["num"] > 0 {
					vI, _ := strconv.Atoi(myval)
					observation.ValueInteger = vI
					// observation.ValueString = ""
					// Else treat it like a string
				} else {
					observation.ValueString = myval
					// observation.ValueInteger = 0
				}
			}
			reference.Reference = &refPatientId // Observation refers to the /Patient/id
			observation.Subject = reference
			observation.ResourceType = "Observation"
			observation.Status = fhir.ObservationStatus(fhir.ObservationStatusRegistered) // Required
			codableText := myUrn + myval
			codableConcept.Text = &codableText
			observation.Code = codableConcept
			// @TODO To switch after debug
			// Unique ID
			id := uuid.New()
			myUuid := myUrn + id.String()
			myPatientEntry := myUuid + "_patient"
			myObservationEntry := myUuid + "_observation"
			// Patient
			if _, added := Find(patients, patientId); added != true {			
				bundleEntry.Resource, _ = patient.MarshalJSON()
				bundleEntry.FullUrl = &myPatientEntry
				bundle.Entry = append(bundle.Entry, bundleEntry)
				patients = append(patients, patientId)
			}
			// Observation
			// bundleEntry.Id = &mySystem
			bundleEntry.Resource, _ = json.Marshal(observation)
			bundleEntry.FullUrl = &myObservationEntry
			bundle.Entry = append(bundle.Entry, bundleEntry)
			observation = FhirObservation{} // Clear values

		}

	}

	return bundle
}

// Find takes a slice and looks for an element in it. If found it will
// return it's key, otherwise it will return -1 and a bool of false.
func Find(slice []string, val string) (int, bool) {
    for i, item := range slice {
        if item == val {
            return i, true
        }
    }
    return -1, false
}
