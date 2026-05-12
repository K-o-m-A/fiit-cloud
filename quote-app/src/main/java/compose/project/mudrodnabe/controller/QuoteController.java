package compose.project.mudrodnabe.controller;

import compose.project.mudrodnabe.domain.QuoteDto;
import compose.project.mudrodnabe.service.QuoteService;
import lombok.extern.slf4j.Slf4j;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@Slf4j
@RestController
public class QuoteController {

    private final QuoteService quoteService;

    public QuoteController(QuoteService quoteService) {
        this.quoteService = quoteService;
    }

    @GetMapping("/quote")
    public QuoteDto getQuote() {
        log.info("Received GET /quote request");
        log.debug("Delegating to QuoteService.getQuote()");
        QuoteDto dto = quoteService.getQuote();
        log.debug("Returning response payload: {}", dto);
        return dto;
    }
}
