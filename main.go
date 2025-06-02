package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net/http" // New import for standard HTTP client
    "os"
    "strconv"
    "time"

    "github.com/andybalholm/brotli"
    "github.com/gocolly/colly"
)

var finalData []interface{}
var dynamicResponse map[string]interface{}
var domainName string = "https://www.thewhiskyexchange.com"

const apiToken string = "tweApiToken"

// Airtable structure - Adjusted for array of records
type AirtablePayload struct {
    Records []AirtableRecord `json:"records"`
}

type AirtableRecord struct {
    Fields AirtableFields `json:"fields"`
}

type AirtableFields struct {
    SKU                 string  `json:"SKU"`
    Name                string  `json:"Name"`
    Price               float64 `json:"Price"`       // Changed to float64 as per Postman test
    ExVatPrice          float64 `json:"ExVATPrice"`  // Changed to float64, JSON tag corrected to ExVATPrice
    ABV                 string  `json:"ABV"`
    Size                string  `json:"Size"`
    Description         string  `json:"Description"`
    ImageUrl            string  `json:"Image URL"`
    ProductUrl          string  `json:"Product URL"`
    ScrapeDate          string  `json:"ScrapedDate"` // JSON tag corrected to ScrapedDate
    IsActive            string    `json:"isActive"`
    MaxOrderQuantity    float64  `json:"MaxOrderQuantity"`
    Manufacturer        string  `json:"Manufacturer"`
    Brand               string  `json:"Brand"`
    MasterCategoryName  string  `json:"MasterCategoryName"`
    CategoryName        string  `json:"CategoryName"`
    Weight              float64  `json:"Weight"`
    StockLevel          float64  `json:"StockLevel"`
    StockControl        float64  `json:"StockControl"`
    IsOutOfStock        string    `json:"isOutofStock"`
}

// Request Payload Structures (for creatingPayload)
type RequestPayload struct {
    Model Model `json:"model"`
}

type FilteringCriterias struct {
    CategoryTagsToFilterBy string      `json:"CategoryTagsToFilterBy"`
    CategoryIdsToFilterBy  string      `json:"CategoryIdsToFilterBy"`
    BrandIdsToFilterBy     string      `json:"BrandIdsToFilterBy"`
    BuyListIdsToFilterBy   string      `json:"BuyListIdsToFilterBy"`
    SearchTextToFilterBy   string      `json:"SearchTextToFilterBy"`
    URLWhereToDisplay      string      `json:"urlwheretodisplay"`
    ExcludeCategoryTags    string      `json:"ExcludeCategoryTags"`
    ExcludeCategoryIds     string      `json:"ExcludeCategoryIds"`
    ExcludeBrandIds        string      `json:"ExcludeBrandIds"`
    ExcludeBuyListIds      string      `json:"ExcludeBuyListIds"`
    BottlingStatus         string      `json:"BottlingStatus"`
    Category               string      `json:"Category"`
    Country                string      `json:"Country"`
    Region                 string      `json:"Region"`
    Author                 string      `json:"Author"`
    Brand                  string      `json:"Brand"`
    GrapeVariety           string      `json:"GrapeVariety"`
    FlavourProfile         string      `json:"FlavourProfile"`
    Age                    string      `json:"Age"`
    Vintage                string      `json:"Vintage"`
    Type                   string      `json:"Type"`
    Style                  string      `json:"Style"`
    CaskType               string      `json:"CaskType"`
    SingleCask             string      `json:"SingleCask"`
    Bottler                string      `json:"Bottler"`
    Series                 string      `json:"Series"`
    Strength               string      `json:"Strength"`
    Size                   string      `json:"Size"`
    Certification          string      `json:"Certification"`
    Sustainability         string      `json:"Sustainability"`
    AgedAtOrigin           string      `json:"AgedAtOrigin"`
    LimitedEdition         string      `json:"LimitedEdition"`
    FoodPairing            string      `json:"FoodPairing"`
    Colouring              string      `json:"Colouring"`
    Flavour                string      `json:"Flavour"`
    IsOnOffer              bool        `json:"IsOnOffer"`
    IncludeOutOfStock      bool        `json:"IncludeOutOfStock"`
    Price                  interface{} `json:"Price"` // Use interface{} for null
}

type DisplaySettings struct {
    PageNumber                int    `json:"PageNumber"`
    ViewMode                  string `json:"ViewMode"`
    PageSize                  string `json:"PageSize"`
    SortingOrder              string `json:"SortingOrder"`
    AnalyticsTrackingCategory string `json:"AnalyticsTrackingCategory"`
}

type DataReturnedSettings struct {
    RemoveSelectedFiltersFromFiltersData bool `json:"RemoveSelectedFiltersFromFiltersData"`
    ReturnArrayOfProductDataForGA4       bool `json:"ReturnArrayOfProductDataForGA4"`
    ReturnArrayOfProducts                bool `json:"ReturnArrayOfProducts"`
    ReturnProductListHtml                bool `json:"ReturnProductListHtml"`
}

type Model struct {
    FilteringCriterias      FilteringCriterias   `json:"FilteringCriterias"`
    DisplaySettings         DisplaySettings      `json:"DisplaySettings"`
    CurrentCustomerSettings string               `json:"CurrentCustomerSettings"`
    ApiToken                string               `json:"ApiToken"`
    DataReturnedSettings    DataReturnedSettings `json:"DataReturnedSettings"`
}

func removeFile(filename string) {
    _, err := os.Stat(filename)

    if err != nil {
        if os.IsNotExist(err) {
            fmt.Printf("File '%s' does not exist. Nothing to remove.\n", filename)
        } else {
            fmt.Printf("Error checking file '%s': %v\n", filename, err)
        }
        return
    }

    fmt.Printf("File '%s' exists. Attempting to remove...\n", filename)
    err = os.Remove(filename)
    if err != nil {
        fmt.Printf("Error removing file '%s': %v\n", filename, err)
        return
    }

    fmt.Printf("File '%s' removed successfully.\n", filename)
}

// createAirtablePayload is no longer used directly as `manipulateData` doesn't push to Airtable.
// Its logic is mostly moved to `extractAirtableFields` for batching.
/* func createAirtablePayload(singleProduct map[string]interface{}) []byte {
    // ... (logic from previous version)
} */

// extractAirtableFields helper function now directly builds the AirtableFields struct
func extractAirtableFields(singleProduct map[string]interface{}) AirtableFields {
    productIDStr := ""
    // Assuming ProductID is already a string in singleProduct after manipulateData
    if id, ok := singleProduct["ProductID"].(string); ok {
        productIDStr = id
    } else if idFloat, ok := singleProduct["ProductID"].(float64); ok {
        productIDStr = strconv.FormatFloat(idFloat, 'f', -1, 64)
        log.Printf("Warning: ProductID for %v was float64 (value: %f) in extractAirtableFields. Converted to string: %s", singleProduct["Name"], idFloat, productIDStr)
    } else {
        log.Printf("Warning: ProductID for %v is not string or float64 (type %T) in extractAirtableFields. Defaulting to empty string for SKU.", singleProduct["Name"], singleProduct["ProductID"])
    }

    priceFloat := 0.0
    if priceVal, ok := singleProduct["SalesPrice"].(float64); ok {
        priceFloat = priceVal
    } else if priceStr, ok := singleProduct["SalesPrice"].(string); ok {
        if p, err := strconv.ParseFloat(priceStr, 64); err == nil {
            priceFloat = p
        } else {
            log.Printf("Warning: Could not parse SalesPrice '%v' to float64 for %v. Using 0.0.", singleProduct["SalesPrice"], singleProduct["Name"])
        }
    }

    exVatPriceFloat := 0.0
    if exVatPriceVal, ok := singleProduct["SalesPriceExVat"].(float64); ok {
        exVatPriceFloat = exVatPriceVal
    } else if exVatPriceStr, ok := singleProduct["SalesPriceExVat"].(string); ok {
        if ep, err := strconv.ParseFloat(exVatPriceStr, 64); err == nil {
            exVatPriceFloat = ep
        } else {
            log.Printf("Warning: Could not parse SalesPriceExVat '%v' to float64 for %v. Using 0.0.", singleProduct["SalesPriceExVat"], singleProduct["Name"])
        }
    }

    abvStr := fmt.Sprintf("%v", singleProduct["StrengthInPC"])
    sizeStr := fmt.Sprintf("%v", singleProduct["SizeInCL"])
    descriptionStr := fmt.Sprintf("%v", singleProduct["Description"])
    productUrlStr := fmt.Sprintf("%v", singleProduct["url"])
    imageUrlStr := fmt.Sprintf("%v", singleProduct["ProductImageUrl"])

    scrapedDateStr := ""
    if t, ok := singleProduct["scrapedDate"].(time.Time); ok {
        scrapedDateStr = t.Format("2006-01-02")
    } else {
        scrapedDateStr = time.Now().UTC().Format("2006-01-02")
        log.Printf("Warning: scrapedDate for %v is not time.Time (type %T). Defaulting to current date (YYYY-MM-DD): %s", singleProduct["Name"], singleProduct["scrapedDate"], scrapedDateStr)
    }

    return AirtableFields{
        SKU:                productIDStr,
        Name:               fmt.Sprintf("%v", singleProduct["Name"]),
        Price:              priceFloat,
        ExVatPrice:         exVatPriceFloat,
        ABV:                abvStr,
        Size:               sizeStr,
        Description:        descriptionStr,
        ProductUrl:         productUrlStr,
        ImageUrl:           imageUrlStr,
        ScrapeDate:         scrapedDateStr,
        IsActive:           fmt.Sprintf("%v", singleProduct["IsActive"]),
        MaxOrderQuantity:   singleProduct["MaxOrderQuantity"].(float64),
        Manufacturer:       fmt.Sprintf("%v", singleProduct["Manufacturer"]),
        Brand:              fmt.Sprintf("%v", singleProduct["Brand"]),
        MasterCategoryName: fmt.Sprintf("%v", singleProduct["MasterCategoryName"]),
        CategoryName:       fmt.Sprintf("%v", singleProduct["CategoryName"]),
        Weight:             singleProduct["Weight"].(float64),
        StockLevel:         singleProduct["StockLevel"].(float64),
        StockControl:       singleProduct["StockControl"].(float64),
        IsOutOfStock:       fmt.Sprintf("%v", singleProduct["IsOutOfStock"]),
    }
}

func createPayload(pageNum int) []byte {
    modelContent := Model{
        FilteringCriterias: FilteringCriterias{
            SearchTextToFilterBy: "s",
            IsOnOffer:            false,
            IncludeOutOfStock:    false,
            Price:                nil,
        },
        DisplaySettings: DisplaySettings{
            PageNumber:                pageNum,
            ViewMode:                  "grid",
            PageSize:                  "1000",
            SortingOrder:              "rdesc",
            AnalyticsTrackingCategory: "Search page",
        },
        CurrentCustomerSettings: "eyJDb29raWVzIjoie1wicnR3ZV9zb3J0aW5nXCI6XCJleHByPXJkZXNjXCIsXCJydHdlX3BhZ2luZ1wiOlwicGFnZXNpemU9MjRcIixcInJ0d2Vfdmlld21vZGVcIjpcIm1vZGU9Z3JpZFwifSJ9",
        ApiToken:                apiToken,
        DataReturnedSettings: DataReturnedSettings{
            RemoveSelectedFiltersFromFiltersData: false,
            ReturnArrayOfProductDataForGA4:       false,
            ReturnArrayOfProducts:                true,
            ReturnProductListHtml:                false,
        },
    }

    requestPayload := RequestPayload{
        Model: modelContent,
    }

    payload, err := json.Marshal(requestPayload)
    if err != nil {
        log.Fatalf("Error marshalling JSON payload: %v", err)
    }
    return payload
}

func manipulateData(collector *colly.Collector, r *colly.Response) {
    var responseBodyToProcess []byte = r.Body
    brotliReader := brotli.NewReader(bytes.NewReader(r.Body))
    decompressedData, err := ioutil.ReadAll(brotliReader)
    if err != nil {
        log.Printf("Error during Brotli decompression for %s: %v", r.Request.URL, err)
    } else {
        fmt.Printf("Brotli decompression successful. Original size: %d, Decompressed size: %d\n", len(r.Body), len(decompressedData))
        responseBodyToProcess = decompressedData
    }

    // For debugging raw response
    // fmt.Println("Raw API Response:", string(responseBodyToProcess))

    dynamicResponse = make(map[string]interface{})
    err = json.Unmarshal(responseBodyToProcess, &dynamicResponse)
    if err != nil {
        log.Printf("Error unmarshalling response body for %s: %v", r.Request.URL, err)
        return
    }

    if productsVal, ok := dynamicResponse["Products"]; ok {
        if productList, isSlice := productsVal.([]interface{}); isSlice {
            for i := range productList {
                if singleProduct, ok := productList[i].(map[string]interface{}); ok {

                    productID := ""
                    // Robust ProductID conversion to string
                    if id, ok := singleProduct["ProductID"].(float64); ok {
                        productID = strconv.FormatFloat(id, 'f', -1, 64)
                    } else if idInt, ok := singleProduct["ProductID"].(int); ok { // Handle direct int
                        productID = strconv.Itoa(idInt)
                    } else if idStr, ok := singleProduct["ProductID"].(string); ok {
                        productID = idStr
                    } else {
                        log.Printf("Warning: ProductID for product '%v' is not float64, int, or string (type %T). Skipping URL generation for this product.", singleProduct["Name"], singleProduct["ProductID"])
                        continue // Skip this product if ProductID is unresolvable
                    }
                    singleProduct["ProductID"] = productID // Ensure ProductID is a string in the map for consistency

                    singleProduct["url"] = domainName + "/p/" + productID
                    singleProduct["scrapedDate"] = time.Now().UTC()

                    fmt.Printf("Collected product: %v (SKU: %s)\n", singleProduct["Name"], productID)
                    finalData = append(finalData, singleProduct)

                    // NO AIRTABLE UPLOAD HERE
                    // The `isAirtableRequest` flag and subsequent PostRaw calls are removed from here.
                }
            }
        }
    }
}

// uploadDataToAirtable is a new function to handle sending data in batches

func uploadDataToAirtable() {
    if len(finalData) == 0 {
        fmt.Println("No data to upload to Airtable.")
        return
    }

    fmt.Printf("Starting Airtable upload for %d collected products...\n", len(finalData))
    const airtableBatchSize = 10 // Airtable allows max 10 records per create request

    client := &http.Client{Timeout: 30 * time.Second} // Use a single client with a timeout

    for i := 0; i < len(finalData); i += airtableBatchSize {
        end := i + airtableBatchSize
        if end > len(finalData) {
            end = len(finalData)
        }
        batch := finalData[i:end]

        var records []AirtableRecord // This slice holds AirtableRecord structs
        for _, product := range batch {
            singleProductMap, ok := product.(map[string]interface{})
            if !ok {
                log.Println("Error: product in finalData is not map[string]interface{}. Skipping batch record.")
                continue
            }
            // THIS IS THE FIX: Create an AirtableRecord and assign the fields
            records = append(records, AirtableRecord{Fields: extractAirtableFields(singleProductMap)})
        }

        if len(records) > 0 {
            payload := AirtablePayload{Records: records}
            payloadBytes, err := json.Marshal(payload)
            if err != nil {
                log.Printf("Error marshalling Airtable batch payload: %v", err)
                continue
            }

            airtableURL := "airtableTableURL"
            req, err := http.NewRequest("POST", airtableURL, bytes.NewBuffer(payloadBytes))
            if err != nil {
                log.Printf("Error creating Airtable request: %v", err)
                continue
            }
            req.Header.Set("Content-Type", "application/json")
            req.Header.Set("Authorization", "Bearer airtableAPIToken")
            req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36") // Include user agent

            resp, err := client.Do(req)
            if err != nil {
                log.Printf("Error sending batch to Airtable: %v", err)
                continue
            }
            defer resp.Body.Close()

            bodyBytes, _ := ioutil.ReadAll(resp.Body) // Read response body for debugging
            if resp.StatusCode >= 200 && resp.StatusCode < 300 {
                fmt.Printf("Successfully uploaded batch %d-%d to Airtable. Response Status: %d\n", i, end-1, resp.StatusCode)
            } else {
                log.Printf("Airtable API returned non-OK status %d for batch %d-%d. Response: %s", resp.StatusCode, i, end-1, string(bodyBytes))
            }
        }
        time.Sleep(250 * time.Millisecond) // Adhere to Airtable rate limit (5 requests/sec = 200ms per request. Add a small buffer)
    }
    fmt.Println("Airtable upload finished.")
}

func main() {

    removeFile("output.json")

    c := colly.NewCollector(
        colly.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"),
    )

    c.OnResponse(func(r *colly.Response) {
        fmt.Printf("Visited %s\n", r.Request.URL)
        manipulateData(c, r)

        totalPages := 1
        currentPage := 1

        // Robustly get TotalPages
        if tpVal, ok := dynamicResponse["TotalPages"]; ok {
            switch v := tpVal.(type) {
            case float64:
                totalPages = int(v)
            case int:
                totalPages = v
            case string:
                if parsedTp, err := strconv.Atoi(v); err == nil {
                    totalPages = parsedTp
                } else {
                    log.Printf("Warning: Could not parse 'TotalPages' string '%s' to int for %s. Defaulting to 1.", v, r.Request.URL)
                }
            default:
                log.Printf("Warning: 'TotalPages' has unexpected type %T for %s. Defaulting to 1.", tpVal, r.Request.URL)
            }
        } else {
            log.Printf("Warning: 'TotalPages' not found for %s. Defaulting to 1.", r.Request.URL)
        }

        // Robustly get CurrentPage
        if cpVal, ok := dynamicResponse["CurrentPage"]; ok {
            switch v := cpVal.(type) {
            case float64:
                currentPage = int(v)
            case int:
                currentPage = v
            case string:
                if parsedCp, err := strconv.Atoi(v); err == nil {
                    currentPage = parsedCp
                } else {
                    log.Printf("Warning: Could not parse 'CurrentPage' string '%s' to int for %s. Defaulting to 1.", v, r.Request.URL)
                }
            default:
                log.Printf("Warning: 'CurrentPage' has unexpected type %T for %s. Defaulting to 1.", cpVal, r.Request.URL)
            }
        } else {
            log.Printf("Warning: 'CurrentPage' not found for %s. Defaulting to 1.", r.Request.URL)
        }

        if currentPage < totalPages {
            nextPage := currentPage + 1
            fmt.Printf("Queuing next page %d (CurrentPage: %d, TotalPages: %d)...\n", nextPage, currentPage, totalPages)
            err := c.PostRaw(domainName+"/api/product/productlistdata", createPayload(nextPage))
            if err != nil {
                fmt.Printf("Error queuing next page %d: %v\n", nextPage, err)
            }
        } else {
            fmt.Println("Last page processed. All data collected.")
            // Write to file after all pages are collected
            jsonDataBytes, err := json.MarshalIndent(finalData, "", "  ")
            if err != nil {
                log.Printf("Error marshalling finalData to JSON for file: %v", err)
                return
            }
            err = ioutil.WriteFile("output.json", jsonDataBytes, 0644)
            if err != nil {
                log.Printf("Error writing output.json: %v", err)
            } else {
                fmt.Println("Successfully wrote response data to output.json")
            }

            // Call Airtable upload ONLY AFTER ALL DATA IS COLLECTED
            uploadDataToAirtable()
        }
    })

    c.OnError(func(r *colly.Response, err error) {
        fmt.Printf("Request to URL: %s failed with error: %v. Response Status: %d\n", r.Request.URL, err, r.StatusCode)
    })

    c.OnRequest(func(r *colly.Request) {
        fmt.Println("Visiting", r.URL.String())
        fmt.Println("Method", r.Method)

        // Set base headers for all requests (initial product list API call and pagination)
        r.Headers.Set("Accept", "*/*")
        r.Headers.Set("Content-Type", "application/json; charset=UTF-8")
        r.Headers.Set("Accept-Encoding", "gzip, deflate, br")
        r.Headers.Set("Connection", "keep-alive")
        r.Headers.Set("Apitoken", `"`+apiToken+`"`)
        r.Headers.Set("Origin", domainName)
        r.Headers.Set("Referer", domainName)
        r.Headers.Set("Cookie", "ASP.NET_SessionId_Live=k13suy13xkup4y3i15lrfkrc; __tweuid=f57d2b00186743408164acde1d6a4708; startedat=29/05/2025 15:42:57; csrf_token=09d10a14-b1c2-4197-99d8-5837b46bc431; _gcl_au=1.1.1818240984.1748529781; _ga=GA1.1.1922052710.1748529782; FPID=FPID2.2.dyVBg1I37L9ADBR7ukAy4r2JNA40EbOc3l1Is2mEUcc%3D.1748529782; FPLC=J%2BUy%2BjlM%2FKzgRHT%2BUoXkDLj1e8TOdUIqYrNoKF4tndL6oJiOOA%2Fe3CvXvG1P6wM%2B7MsRhWHy8S3qPPugGG7yGSCUqYH1%2BqsT5edGHQfnK%2Flxlr0VYcTY4ChMFpz0MA%3D%3D; _gtmeec=e30%3D; _pin_unauth=dWlkPU5EbGpZV0ZrTTJVdFpUZ3lNQzAwWVRabUxUbGxORFV0TmpGa1kyWTFaREkzWlRneA; _fbp=fb.1.1748529782795.518524410768218357; lantern=819ea3d4-4206-4010-b18b-f4d41cc509ca; _hjSessionUser_3524759=eyJpZCI6ImE4NzJjNDIzLTQzZDctNWEzZi05OTQ5LWMwYzdmNGM5MzhmOCIsImNyZWF0ZWQiOjE3NDg1Mjk4NDIyNTMsImV4aXN0aW5nIjp0cnVlfQ==; __zlcmid=1RtneNrLCzqxS0X; twe_recently_viewed_products=idlist=29388,23771,; rtwe_paging=pagesize=24; rtwe_sorting=expr=rdesc; rtwe_viewmode=mode=grid; CloseTrustBar=true; __cf_bm=dj1gLm__Ry_B97f5rCaHu5ibwWYckOe2dHI7.JZJwAg-1748549662-1.0.1.1-7daMkVDRtkvEe4qgqVGa9ZeVmu8cWxetq.cKhUl9HnhMlHUKgJ.27SFsWSwrrwdcQ4OZIu8GY0qpyCUXR3euYADoQXIkg80tom7yowZE9oo; cf_clearance=YPQDxYDy5sinyhswcwAC0sbGPYfHyeilgL1Jq52rzcY-1748549663-1.2.1.1-aD0WlfF3vu7Jz2m_j4C7QG0HGQzEJDFE190c.oVYcPYIm3v4Y5VAFmHxJOgo4bzpHTxTfQFvvogBAeFNfY4gxlEcSD8MmVCCqKMb6zmb6WEuhO20hH7TtM12rsrl1e4i_EbtL2ksZdBBcrb_7mk35_5j2uJOEWLSw9ksTXGq_.uQN7Mp3fm919JnzZRCKkL0f5NpqDJNUZ4qGucLk9ynB5jcDjC8Jqoqh3yod.W.bK4M4DH2FOoTEN3fEjx7Tr9e_mAlazwmRmNHw9Rt8n1sgCNfTVCsM4BerA3_8bEjLdAymgq57TKCnx9DdHKFAbGHMLoCpPgtYB.rQB1M3zANLdtH7GuVOP64kzZkjVTrWb.Q; _hjSession_3524759=eyJpZCI6ImE5YTdjOTFhLTQwNjgtNGZmNi04ZjQyLTNhODczNWQwNjQ3NiIsImMiOjE3NDg1NDk3MjM3MzIsInMiOjEsInIiOjEsInNiIjowLCJzciI6MCwic2UiOjAsImZzIjowLCJzcCI6MH0=; ometria=2_cid%3DhERZhnJoZtX3GE8L%26nses%3D4%26osts%3D1748529782%26sid%3Df15e3b138QWz1uwaeidGH%26npv%3D1%26tids%3D%26ecamp%3D%26src%3Dupwork.com%257C%257C%257C%257C%257C%257C20237%26osrc%3Dupwork.com%257C%257C%257C%257C%257C%257C20237%26slt%3D1748549944; _uetsid=3964d3c03c9b11f0bd1839f24fc1b15d; _uetvid=39650f403c9b11f0b23741728c04542f; ABTastySession=mrasn=&lp=https%253A%252F%252Fwww.thewhiskyexchange.com%252F; ABTasty=uid=j3pv5vdz5e43cv9n&fst=1748529782520&pst=1748541124750&cst=1748549663519&ns=5&pvt=50&pvis=9&th=1435703.1784616.50.9.5.1.1748529782907.1748549944667.0.5&eas=; _ga_53RV91M60Z=GS2.1.s1748549662$o5$g1$t1748549950$j36$l0$h319685231; _ga_43BPYNRML5=GS2.1.s1748549662$o5$g1$t1748549950$j36$l0$h0")

        // No longer setting conditional headers for Airtable here
    })

    err := c.PostRaw(domainName+"/api/product/productlistdata", createPayload(1))

    if err != nil {
        fmt.Println("Initial PostRaw request failed:", err)
    }

    c.Wait() // Wait for all Colly scraping operations to complete
}